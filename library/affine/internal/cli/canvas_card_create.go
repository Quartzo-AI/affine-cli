package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"affine-pp-cli/internal/canvaswrite"
	"affine-pp-cli/internal/config"

	"github.com/spf13/cobra"
)

type canvasCardCreateSpec struct {
	ID          string                      `json:"id"`
	X           float64                     `json:"x"`
	Y           float64                     `json:"y"`
	W           float64                     `json:"w"`
	H           float64                     `json:"h"`
	DisplayMode string                      `json:"display_mode,omitempty"`
	Index       string                      `json:"index,omitempty"`
	Background  string                      `json:"background,omitempty"`
	FrameID     string                      `json:"frame_id,omitempty"`
	Markdown    string                      `json:"markdown,omitempty"`
	Paragraphs  []canvaswrite.CardParagraph `json:"paragraphs,omitempty"`
}

type canvasCardCreateSpecFile struct {
	DocID string                 `json:"doc_id,omitempty"`
	Cards []canvasCardCreateSpec `json:"cards"`
}

type canvasCardCreateFlags struct {
	specPath    string
	markdown    string
	textFile    string
	workspaceID string
	docID       string
	backupDir   string
	apply       bool
	live        bool
	card        canvasCardCreateSpec
}

func newCanvasCardCreateCmd(flags *rootFlags) *cobra.Command {
	opts := &canvasCardCreateFlags{}
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create AFFiNE note cards on an edgeless doc through the gated Y.js write layer",
		Long: "Create AFFiNE note cards on an edgeless doc. Without --apply/--live the command only emits a validated " +
			"canvas_transform plan and touches nothing. A live apply runs the same gates as every other canvas write: " +
			"pre-integrity, backup and state vector, local Y.js mutation, post-local integrity, delta push, reload verification and post-integrity.",
		Example: "  affine-pp-cli canvas card create --doc <doc-id> --id card-a --x 0 --y 0 --w 360 --h 220 --text \"### Card\\n\\nBody\" --json\n" +
			"  affine-pp-cli canvas card create --spec cards.json --doc <doc-id> --json\n" +
			"  affine-pp-cli canvas card create --spec cards.json --live --workspace <workspace-id> --doc <doc-id> --backup-dir ./backups --yes --json",
		Annotations: map[string]string{
			"pp:requires-tier": "affine-workspace-fixture",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			createOpts, err := buildCanvasCardCreateOptions(opts, cmd.InOrStdin())
			if err != nil {
				return err
			}
			plan, err := canvaswrite.BuildCardCreatePlan(createOpts)
			if err != nil {
				return err
			}
			liveApply := (opts.apply || opts.live) && !flags.dryRun
			if !liveApply {
				return writeJSON(cmd.OutOrStdout(), map[string]any{
					"dry_run":               true,
					"plan_type":             plan.PlanType,
					"plan_id":               plan.PlanID,
					"doc_id":                plan.DocID,
					"affected_ids":          plan.AffectedIDs,
					"created_block_ids":     canvaswrite.CardCreateIDs(plan.Operations),
					"operations":            plan.Operations,
					"semantic_diff_preview": canvaswrite.CardCreateDiffPreview(plan.Operations),
					"live_write_supported":  true,
					"live_write_requires":   []string{"--live", "--workspace", "--doc", "--backup-dir", "--yes"},
				})
			}
			if !flags.yes {
				return fmt.Errorf("confirmation required: pass --yes for live canvas apply")
			}
			applyOpts := canvaswrite.TransformApplyOptions{WorkspaceID: opts.workspaceID, DocID: opts.docID, BackupDir: opts.backupDir}
			if err := canvaswrite.ValidateTransformApply(plan, applyOpts); err != nil {
				return err
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return err
			}
			result, err := canvaswrite.ApplyTransformPlan(cfg, plan, applyOpts)
			if err != nil {
				return err
			}
			return writeJSON(cmd.OutOrStdout(), result)
		},
	}
	cmd.Flags().StringVar(&opts.specPath, "spec", "", "JSON card spec file with a cards array; use - for stdin")
	cmd.Flags().StringVar(&opts.card.ID, "id", "", "Block ID for the created card")
	cmd.Flags().Float64Var(&opts.card.X, "x", 0, "Card x position")
	cmd.Flags().Float64Var(&opts.card.Y, "y", 0, "Card y position")
	cmd.Flags().Float64Var(&opts.card.W, "w", 360, "Card width")
	cmd.Flags().Float64Var(&opts.card.H, "h", 220, "Card height")
	cmd.Flags().StringVar(&opts.card.DisplayMode, "display-mode", "edgeless", "Card display mode: edgeless, both or page")
	cmd.Flags().StringVar(&opts.card.Background, "background", "", "Card background value")
	cmd.Flags().StringVar(&opts.card.FrameID, "frame", "", "Containing frame ID to register the card in")
	cmd.Flags().StringVar(&opts.markdown, "text", "", "Card markdown; headings become h1-h6 leaf paragraphs")
	cmd.Flags().StringVar(&opts.textFile, "text-file", "", "UTF-8 markdown file with the card content")
	cmd.Flags().StringVar(&opts.workspaceID, "workspace", "", "AFFiNE workspace ID for live apply")
	cmd.Flags().StringVar(&opts.docID, "doc", "", "AFFiNE document ID")
	cmd.Flags().StringVar(&opts.backupDir, "backup-dir", "", "Directory for before/delta backups when applying")
	cmd.Flags().BoolVar(&opts.apply, "apply", false, "Push the created cards to AFFiNE")
	cmd.Flags().BoolVar(&opts.live, "live", false, "Alias for --apply")
	return cmd
}

func buildCanvasCardCreateOptions(opts *canvasCardCreateFlags, stdin io.Reader) (canvaswrite.CardCreateOptions, error) {
	specs, docID, err := readCanvasCardCreateSpecs(opts, stdin)
	if err != nil {
		return canvaswrite.CardCreateOptions{}, err
	}
	out := canvaswrite.CardCreateOptions{DocID: docID}
	for _, spec := range specs {
		id := strings.TrimSpace(spec.ID)
		if id == "" {
			return canvaswrite.CardCreateOptions{}, fmt.Errorf("every card spec requires an id")
		}
		paragraphs := spec.Paragraphs
		if len(paragraphs) == 0 {
			paragraphs = canvaswrite.ParseCardMarkdown(spec.Markdown)
		}
		if len(paragraphs) == 0 {
			return canvaswrite.CardCreateOptions{}, fmt.Errorf("card %q has no text; pass --text, --text-file or paragraphs", id)
		}
		out.IDs = append(out.IDs, id)
		out.Cards = append(out.Cards, canvaswrite.CardCreateSpec{
			X:           spec.X,
			Y:           spec.Y,
			W:           spec.W,
			H:           spec.H,
			DisplayMode: spec.DisplayMode,
			Index:       spec.Index,
			Background:  spec.Background,
			FrameID:     spec.FrameID,
			Paragraphs:  paragraphs,
		})
	}
	return out, nil
}

func readCanvasCardCreateSpecs(opts *canvasCardCreateFlags, stdin io.Reader) ([]canvasCardCreateSpec, string, error) {
	if opts.specPath != "" {
		var data []byte
		var err error
		if opts.specPath == "-" {
			data, err = io.ReadAll(stdin)
		} else {
			data, err = os.ReadFile(opts.specPath)
		}
		if err != nil {
			return nil, "", err
		}
		var file canvasCardCreateSpecFile
		if err := json.Unmarshal(data, &file); err != nil {
			return nil, "", fmt.Errorf("parsing canvas card spec JSON: %w", err)
		}
		if len(file.Cards) == 0 {
			return nil, "", fmt.Errorf("canvas card spec has no cards")
		}
		docID := opts.docID
		if docID == "" {
			docID = file.DocID
		}
		return file.Cards, docID, nil
	}
	card := opts.card
	markdown := opts.markdown
	if opts.textFile != "" {
		data, err := os.ReadFile(opts.textFile)
		if err != nil {
			return nil, "", err
		}
		markdown = string(data)
	}
	card.Markdown = strings.ReplaceAll(markdown, "\\n", "\n")
	return []canvasCardCreateSpec{card}, opts.docID, nil
}
