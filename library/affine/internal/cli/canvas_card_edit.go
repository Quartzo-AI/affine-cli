package cli

import (
	"affine-pp-cli/internal/canvaswrite"
	"affine-pp-cli/internal/config"
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"os"
)

func newCanvasCardEditCmd(flags *rootFlags) *cobra.Command {
	var specPath, workspaceID, docID, backupDir string
	var apply bool
	cmd := &cobra.Command{Use: "edit", Short: "Edit card text and frames without replacing media; plans by default",
		RunE: func(cmd *cobra.Command, args []string) error {
			raw, err := os.ReadFile(specPath)
			if err != nil {
				return err
			}
			var spec struct {
				DocID      string                           `json:"doc_id"`
				Operations []canvaswrite.TransformOperation `json:"operations"`
			}
			if err = json.Unmarshal(raw, &spec); err != nil {
				return err
			}
			if docID == "" {
				docID = spec.DocID
			} else if spec.DocID != "" && spec.DocID != docID {
				return fmt.Errorf("spec doc_id does not match --doc")
			}
			plan, err := canvaswrite.BuildCanvasEditPlan(docID, spec.Operations)
			if err != nil {
				return err
			}
			if !apply || flags.dryRun {
				return writeJSON(cmd.OutOrStdout(), plan)
			}
			if !flags.yes {
				return fmt.Errorf("live edit requires --yes")
			}
			opts := canvaswrite.TransformApplyOptions{WorkspaceID: workspaceID, DocID: docID, BackupDir: backupDir}
			if err = canvaswrite.ValidateTransformApply(plan, opts); err != nil {
				return err
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return err
			}
			result, err := canvaswrite.ApplyTransformPlan(cfg, plan, opts)
			if err != nil {
				return err
			}
			return writeJSON(cmd.OutOrStdout(), result)
		}}
	cmd.Flags().StringVar(&specPath, "spec", "", "JSON edit manifest (doc_id, operations)")
	cmd.Flags().StringVar(&workspaceID, "workspace", "", "Workspace for live apply")
	cmd.Flags().StringVar(&docID, "doc", "", "Document ID")
	cmd.Flags().StringVar(&backupDir, "backup-dir", "", "Before/delta backup directory")
	cmd.Flags().BoolVar(&apply, "apply", false, "Apply the reviewed edit manifest")
	return cmd
}
