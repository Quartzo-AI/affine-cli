package cli

import (
	"affine-pp-cli/internal/config"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func newCanvasCardUploadImageCmd(flags *rootFlags) *cobra.Command {
	var file, workspace string
	var apply bool
	cmd := &cobra.Command{Use: "upload-image", Short: "Upload an image blob through AFFiNE GraphQL; dry-run by default", RunE: func(cmd *cobra.Command, args []string) error {
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		if len(data) > 10<<20 {
			return fmt.Errorf("image exceeds 10 MiB")
		}
		mime := http.DetectContentType(data)
		if !strings.HasPrefix(mime, "image/") {
			return fmt.Errorf("file is not a raster image")
		}
		if workspace == "" {
			return fmt.Errorf("--workspace required")
		}
		if !apply || flags.dryRun {
			return writeJSON(cmd.OutOrStdout(), map[string]any{"dry_run": true, "file": file, "mime": mime, "bytes": len(data), "workspace": workspace})
		}
		if !flags.yes {
			return fmt.Errorf("upload requires --yes")
		}
		cfg, err := config.Load(flags.configPath)
		if err != nil {
			return err
		}
		id, err := uploadCanvasImage(cfg, workspace, filepath.Base(file), mime, data)
		if err != nil {
			return err
		}
		return writeJSON(cmd.OutOrStdout(), map[string]any{"applied": true, "source_id": id, "bytes": len(data)})
	}}
	cmd.Flags().StringVar(&file, "file", "", "Local raster image")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace ID")
	cmd.Flags().BoolVar(&apply, "apply", false, "Upload the reviewed image")
	return cmd
}

func uploadCanvasImage(cfg *config.Config, workspace, name, mime string, data []byte) (string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	operation, _ := json.Marshal(map[string]any{"query": "mutation($workspaceId: String!, $blob: Upload!) { setBlob(workspaceId:$workspaceId, blob:$blob) }", "variables": map[string]any{"workspaceId": workspace, "blob": nil}})
	if err := writer.WriteField("operations", string(operation)); err != nil {
		return "", err
	}
	if err := writer.WriteField("map", `{"0":["variables.blob"]}`); err != nil {
		return "", err
	}
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="0"; filename=%q`, name))
	header.Set("Content-Type", mime)
	part, err := writer.CreatePart(header)
	if err != nil {
		return "", err
	}
	if _, err = part.Write(data); err != nil {
		return "", err
	}
	if err = writer.Close(); err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(cfg.BaseURL, "/")+"/graphql", &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", cfg.AuthHeader())
	req.Header.Set("Apollo-Require-Preflight", "true")
	client := &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("image upload transport failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("image upload HTTP %d", resp.StatusCode)
	}
	var result struct {
		Data struct {
			SetBlob string `json:"setBlob"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil {
		return "", fmt.Errorf("invalid image upload response")
	}
	if len(result.Errors) > 0 || result.Data.SetBlob == "" {
		return "", fmt.Errorf("image upload rejected by GraphQL")
	}
	return result.Data.SetBlob, nil
}
