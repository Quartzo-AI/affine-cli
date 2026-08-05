package cli

// pp:data-source auto
// pp:client-call

import (
	"affine-pp-cli/internal/canvaswrite"
	"affine-pp-cli/internal/config"
	"affine-pp-cli/internal/socketio"
	"affine-pp-cli/internal/yjs"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

type canvasBuildSpec struct {
	Frame       string              `json:"frame"`
	Orientation string              `json:"orientation"`
	Hub         string              `json:"hub"`
	Levels      [][]string          `json:"levels"`
	Connections []canvasConnection  `json:"connections,omitempty"`
	Card        canvasCardSize      `json:"card,omitempty"`
	Spacing     canvasLayoutSpacing `json:"spacing,omitempty"`
	Origin      canvasPoint         `json:"origin,omitempty"`
}

type canvasCardSize struct {
	W float64 `json:"w,omitempty"`
	H float64 `json:"h,omitempty"`
}

type canvasLayoutSpacing struct {
	X float64 `json:"x,omitempty"`
	Y float64 `json:"y,omitempty"`
}

type canvasPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type canvasTextBlock struct {
	ID   string `json:"id,omitempty"`
	Text string `json:"text"`
	Type string `json:"type,omitempty"`
}

type canvasNode struct {
	ID              string            `json:"id"`
	Text            string            `json:"text"`
	TextBlocks      []canvasTextBlock `json:"text_blocks,omitempty"`
	ReplaceChildren bool              `json:"replace_children,omitempty"`
	DisplayMode     string            `json:"display_mode,omitempty"`
	Background      any               `json:"background,omitempty"`
	Edgeless        map[string]any    `json:"edgeless,omitempty"`
	Role            string            `json:"role,omitempty"`
	Level           int               `json:"level"`
	X               float64           `json:"x"`
	Y               float64           `json:"y"`
	W               float64           `json:"w"`
	H               float64           `json:"h"`
}

type canvasConnection struct {
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type,omitempty"`
}

type canvasPlan struct {
	Frame       string             `json:"frame"`
	Orientation string             `json:"orientation"`
	Nodes       []canvasNode       `json:"nodes"`
	Connections []canvasConnection `json:"connections"`
	Warnings    []string           `json:"warnings,omitempty"`
}

type canvasLayoutApplyResult struct {
	PlanType           string                         `json:"plan_type"`
	DocID              string                         `json:"doc_id"`
	DryRun             bool                           `json:"dry_run"`
	Applied            bool                           `json:"applied"`
	BackupDir          string                         `json:"backup_dir"`
	UpsertedNodes      []string                       `json:"upserted_nodes"`
	UpsertedConnectors []string                       `json:"upserted_connectors"`
	Before             canvaswrite.DocIntegrityResult `json:"before"`
	After              canvaswrite.DocIntegrityResult `json:"after"`
	Proof              canvaswrite.TransformProof     `json:"proof"`
}

type canvasModel struct {
	Nodes       []map[string]any `json:"nodes"`
	Connections []map[string]any `json:"connections"`
	Warnings    []string         `json:"warnings,omitempty"`
}

type canvasValidationReport struct {
	OK         bool                    `json:"ok"`
	IssueCount int                     `json:"issue_count"`
	Issues     []canvasValidationIssue `json:"issues,omitempty"`
}

type canvasValidationIssue struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	NodeID   string `json:"node_id,omitempty"`
	From     string `json:"from,omitempty"`
	To       string `json:"to,omitempty"`
}

func newCanvasCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "canvas",
		Short: "Plan and inspect AFFiNE canvas layouts",
		Long:  "Plan and inspect AFFiNE canvas layouts. Vertical trees are the default; horizontal layouts are available with --orientation horizontal.",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newCanvasPlanCmd(flags))
	cmd.AddCommand(newCanvasApplyCmd(flags))
	cmd.AddCommand(newCanvasSearchCmd(flags))
	cmd.AddCommand(newCanvasDiffCmd(flags))
	cmd.AddCommand(newCanvasTransformCmd(flags))
	cmd.AddCommand(newCanvasLabelPlanCmd(flags))
	cmd.AddCommand(newCanvasModelCmd(flags))
	cmd.AddCommand(newCanvasValidateCmd(flags))
	cmd.AddCommand(newCanvasExampleCmd())
	cmd.AddCommand(newCanvasCardCmd(flags))
	cmd.AddCommand(newCanvasDocCmd(flags))
	cmd.AddCommand(newCanvasBlockCmd(flags))
	return cmd
}

func newCanvasLabelPlanCmd(flags *rootFlags) *cobra.Command {
	var opts canvaswrite.ConnectorLabelOptions
	var selectorsPath string
	cmd := &cobra.Command{
		Use:   "label-plan",
		Short: "Build a reviewable AFFiNE canvas connector label plan",
		Example: "  affine-pp-cli canvas label-plan --selectors connectors.json --label edge-id=\"Reviewed Label\" --json\n" +
			"  affine-pp-cli canvas label-plan --selectors connectors.json --id edge-id --text \"Reviewed Label\" --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			selectors, err := readCanvasSelectors(selectorsPath, cmd.InOrStdin(), len(opts.IDs) > 0)
			if err != nil {
				return err
			}
			plan, err := canvaswrite.BuildConnectorLabelPlan(selectors, opts)
			if err != nil {
				return err
			}
			return writeJSON(cmd.OutOrStdout(), plan)
		},
	}
	cmd.Flags().StringVar(&selectorsPath, "selectors", "", "canvas search JSON file; use - for stdin")
	cmd.Flags().StringVar(&opts.DocID, "doc", "", "AFFiNE document ID for the connector label plan")
	cmd.Flags().StringSliceVar(&opts.IDs, "id", nil, "Selected connector ID; may be repeated or comma-separated")
	cmd.Flags().StringArrayVar(&opts.Labels, "label", nil, "Connector label as connector-id=text; may be repeated")
	cmd.Flags().StringVar(&opts.Text, "text", "", "Label text to apply to every selected --id")
	cmd.Flags().BoolVar(&opts.AllowEmpty, "allow-empty", false, "Allow empty label text to remove a connector label")
	cmd.Flags().StringVar(&opts.BackupTarget, "backup-target", "", "Expected backup target for later live apply proof")
	return cmd
}

func newCanvasTransformCmd(flags *rootFlags) *cobra.Command {
	var opts canvaswrite.TransformOptions
	var selectorsPath string
	cmd := &cobra.Command{
		Use:   "transform",
		Short: "Build a dry-run AFFiNE canvas transform plan",
		Example: "  affine-pp-cli canvas transform --selectors search.json --move 10,0 --json\n" +
			"  affine-pp-cli canvas transform --id card-a --display-mode embed --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			selectors, err := readCanvasSelectors(selectorsPath, cmd.InOrStdin(), len(opts.IDs) > 0)
			if err != nil {
				return err
			}
			plan, err := canvaswrite.BuildTransformPlan(selectors, opts)
			if err != nil {
				return err
			}
			return writeJSON(cmd.OutOrStdout(), plan)
		},
	}
	cmd.Flags().StringVar(&selectorsPath, "selectors", "", "canvas search JSON file; use - for stdin")
	cmd.Flags().StringVar(&opts.DocID, "doc", "", "AFFiNE document ID for the transform plan")
	cmd.Flags().StringSliceVar(&opts.IDs, "id", nil, "Selected block ID; may be repeated or comma-separated")
	cmd.Flags().StringVar(&opts.Move, "move", "", "Move selected entities by dx,dy")
	cmd.Flags().StringVar(&opts.Resize, "resize", "", "Resize selected entities to width,height")
	cmd.Flags().StringVar(&opts.Align, "align", "", "Align selected entities: left, right, top, bottom, center-x, center-y")
	cmd.Flags().StringVar(&opts.Distribute, "distribute", "", "Distribute selected entities: horizontal or vertical")
	cmd.Flags().StringVar(&opts.DisplayMode, "display-mode", "", "Set selected cards to display mode")
	cmd.Flags().StringArrayVar(&opts.Metadata, "set", nil, "Set metadata as key=value; may be repeated")
	cmd.Flags().StringVar(&opts.BackupTarget, "backup-target", "", "Expected backup target for later live apply proof")
	return cmd
}

func newCanvasDiffCmd(flags *rootFlags) *cobra.Command {
	var opts canvaswrite.DiffOptions
	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Compare two AFFiNE canvas states",
		Example: "  affine-pp-cli canvas diff --before-snapshot old.bin --after-snapshot new.bin --json\n" +
			"  affine-pp-cli canvas diff --workspace <workspace-id> --doc <doc-id> --before-timestamp <ts> --json",
		Annotations: map[string]string{
			"mcp:read-only":    "true",
			"pp:requires-tier": "affine-workspace-fixture",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return err
			}
			result, err := canvaswrite.DiffDoc(cfg, opts)
			if err != nil {
				return err
			}
			return writeJSON(cmd.OutOrStdout(), result)
		},
	}
	cmd.Flags().StringVar(&opts.WorkspaceID, "workspace", "", "AFFiNE workspace ID")
	cmd.Flags().StringVar(&opts.DocID, "doc", "", "AFFiNE document ID")
	cmd.Flags().StringVar(&opts.BeforeSnapshotFile, "before-snapshot", "", "Before-state local AFFiNE Y.js snapshot file")
	cmd.Flags().StringVar(&opts.AfterSnapshotFile, "after-snapshot", "", "After-state local AFFiNE Y.js snapshot file")
	cmd.Flags().StringVar(&opts.BeforeTimestamp, "before-timestamp", "", "Before-state AFFiNE history timestamp")
	cmd.Flags().StringVar(&opts.AfterTimestamp, "after-timestamp", "", "After-state AFFiNE history timestamp; defaults to live when omitted")
	cmd.Flags().IntVar(&opts.TextLimit, "text-limit", 240, "Maximum text length for compared entities")
	return cmd
}

func newCanvasSearchCmd(flags *rootFlags) *cobra.Command {
	var opts canvaswrite.SearchOptions
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search AFFiNE canvas blocks, cards, and connectors",
		Example: "  affine-pp-cli canvas search --snapshot-file doc.bin --text roadmap --json\n" +
			"  affine-pp-cli canvas search --workspace <workspace-id> --doc <doc-id> --flavour affine:note --json",
		Annotations: map[string]string{
			"mcp:read-only":    "true",
			"pp:requires-tier": "affine-workspace-fixture",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return err
			}
			result, err := canvaswrite.SearchDoc(cfg, opts)
			if err != nil {
				return err
			}
			return writeJSON(cmd.OutOrStdout(), result)
		},
	}
	cmd.Flags().StringVar(&opts.WorkspaceID, "workspace", "", "AFFiNE workspace ID")
	cmd.Flags().StringVar(&opts.DocID, "doc", "", "AFFiNE document ID")
	cmd.Flags().StringVar(&opts.SnapshotFile, "snapshot-file", "", "Local AFFiNE Y.js snapshot file to search")
	cmd.Flags().StringVar(&opts.Timestamp, "timestamp", "", "AFFiNE history timestamp to search")
	cmd.Flags().StringVar(&opts.Text, "text", "", "Case-insensitive text substring")
	cmd.Flags().StringVar(&opts.ID, "id", "", "Exact block ID")
	cmd.Flags().StringVar(&opts.Flavour, "flavour", "", "Exact sys:flavour value")
	cmd.Flags().StringVar(&opts.Type, "type", "", "Exact sys:type value")
	cmd.Flags().StringVar(&opts.DisplayMode, "display-mode", "", "Exact note display mode")
	cmd.Flags().StringVar(&opts.SourceMode, "source-mode", "", "Source mode filter: live, snapshot, or history")
	cmd.Flags().StringVar(&opts.Bounds, "bounds", "", "Rectangle filter as x,y,w,h")
	cmd.Flags().StringVar(&opts.ConnectorFrom, "from", "", "Connector source block ID")
	cmd.Flags().StringVar(&opts.ConnectorTo, "to", "", "Connector target block ID")
	cmd.Flags().IntVar(&opts.Limit, "limit", 0, "Maximum matches; 0 returns all")
	cmd.Flags().IntVar(&opts.TextLimit, "text-limit", 240, "Maximum text length for each result")
	return cmd
}

func newCanvasBlockCmd(flags *rootFlags) *cobra.Command {
	var opts canvaswrite.BlockInspectOptions
	cmd := &cobra.Command{
		Use:     "block",
		Short:   "Inspect one AFFiNE block without modifying it",
		Example: "  affine-pp-cli canvas block --workspace <workspace-id> --doc <doc-id> --block <block-id> --json",
		Annotations: map[string]string{
			"mcp:read-only":    "true",
			"pp:requires-tier": "affine-workspace-fixture",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return err
			}
			result, err := canvaswrite.InspectBlock(cfg, opts)
			if err != nil {
				return err
			}
			return writeJSON(cmd.OutOrStdout(), result)
		},
	}
	cmd.Flags().StringVar(&opts.WorkspaceID, "workspace", "", "AFFiNE workspace ID")
	cmd.Flags().StringVar(&opts.DocID, "doc", "", "AFFiNE document ID")
	cmd.Flags().StringVar(&opts.BlockID, "block", "", "Block ID")
	return cmd
}
func newCanvasDocCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doc",
		Short: "Inspect a canvas document",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newCanvasDocAuditCmd(flags))
	cmd.AddCommand(newCanvasDocIntegrityCmd(flags))
	cmd.AddCommand(newCanvasDocRepairCmd(flags))
	return cmd
}

func newCanvasDocIntegrityCmd(flags *rootFlags) *cobra.Command {
	var opts canvaswrite.DocIntegrityOptions
	cmd := &cobra.Command{
		Use:   "integrity",
		Short: "Check structural integrity of one AFFiNE canvas document",
		Example: "  affine-pp-cli canvas doc integrity --workspace <workspace-id> --doc <doc-id> --json\n" +
			"  affine-pp-cli canvas doc integrity --snapshot-file doc.bin --json",
		Annotations: map[string]string{
			"mcp:read-only":    "true",
			"pp:requires-tier": "affine-workspace-fixture",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return err
			}
			result, err := canvaswrite.CheckDocIntegrity(cfg, opts)
			if err != nil {
				return err
			}
			return writeJSON(cmd.OutOrStdout(), result)
		},
	}
	cmd.Flags().StringVar(&opts.WorkspaceID, "workspace", "", "AFFiNE workspace ID")
	cmd.Flags().StringVar(&opts.DocID, "doc", "", "AFFiNE document ID")
	cmd.Flags().StringVar(&opts.SnapshotFile, "snapshot-file", "", "Local AFFiNE Y.js snapshot file to check")
	cmd.Flags().StringVar(&opts.Timestamp, "timestamp", "", "AFFiNE history timestamp to check")
	return cmd
}

func newCanvasDocRepairCmd(flags *rootFlags) *cobra.Command {
	var opts canvaswrite.DocRepairOptions
	cmd := &cobra.Command{
		Use:   "repair",
		Short: "Repair a narrow set of safe AFFiNE canvas document integrity issues",
		Example: "  affine-pp-cli canvas doc repair --workspace <workspace-id> --doc <doc-id> --dry-run --json\n" +
			"  affine-pp-cli canvas doc repair --workspace <workspace-id> --doc <doc-id> --fix connector-blocks --all --apply --backup-dir ./backups --json",
		Annotations: map[string]string{
			"pp:requires-tier": "affine-workspace-fixture",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				opts.Apply = false
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return err
			}
			result, err := canvaswrite.RepairDoc(cfg, opts)
			if err != nil {
				return err
			}
			return writeJSON(cmd.OutOrStdout(), result)
		},
	}
	cmd.Flags().StringVar(&opts.WorkspaceID, "workspace", "", "AFFiNE workspace ID")
	cmd.Flags().StringVar(&opts.DocID, "doc", "", "AFFiNE document ID")
	cmd.Flags().StringVar(&opts.Fix, "fix", "missing-children", "Integrity issue to fix: missing-children or connector-blocks")
	cmd.Flags().StringSliceVar(&opts.IDs, "id", nil, "Repair only selected issue block ID; may be repeated or comma-separated")
	cmd.Flags().StringVar(&opts.BackupDir, "backup-dir", "", "Directory for before/delta backups when applying")
	cmd.Flags().BoolVar(&opts.Apply, "apply", false, "Push the repair update to AFFiNE")
	cmd.Flags().BoolVar(&opts.All, "all", false, "Allow broad repair of every matching issue")
	return cmd
}

func newCanvasDocAuditCmd(flags *rootFlags) *cobra.Command {
	var opts canvaswrite.DocAuditOptions
	var keywords string
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Audit one AFFiNE canvas document without modifying it",
		Example: "  affine-pp-cli canvas doc audit --workspace <workspace-id> --doc <doc-id> --keywords trabalho,comunidade --json\n" +
			"  affine-pp-cli canvas doc audit --snapshot-file doc.bin --json",
		Annotations: map[string]string{
			"mcp:read-only":    "true",
			"pp:requires-tier": "affine-workspace-fixture",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return err
			}
			opts.Keywords = splitCanvasKeywords(keywords)
			result, err := canvaswrite.AuditDoc(cfg, opts)
			if err != nil {
				return err
			}
			return writeJSON(cmd.OutOrStdout(), result)
		},
	}
	cmd.Flags().StringVar(&opts.WorkspaceID, "workspace", "", "AFFiNE workspace ID")
	cmd.Flags().StringVar(&opts.DocID, "doc", "", "AFFiNE document ID")
	cmd.Flags().StringVar(&opts.SnapshotFile, "snapshot-file", "", "Local AFFiNE Y.js snapshot file to audit")
	cmd.Flags().StringVar(&opts.Timestamp, "timestamp", "", "AFFiNE history timestamp to audit")
	cmd.Flags().StringVar(&keywords, "keywords", "", "Comma-separated keywords to count and sample")
	cmd.Flags().IntVar(&opts.TextLimit, "text-limit", 240, "Maximum text length for each match sample")
	return cmd
}

func newCanvasCardCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "card",
		Short: "Create and patch individual canvas cards",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newCanvasCardCreateCmd(flags))
	cmd.AddCommand(newCanvasCardSetImageCmd(flags))
	cmd.AddCommand(newCanvasCardInspectCmd(flags))
	return cmd
}

func newCanvasCardInspectCmd(flags *rootFlags) *cobra.Command {
	var opts canvaswrite.CardInspectOptions
	cmd := &cobra.Command{
		Use:     "inspect",
		Short:   "Inspect one canvas card's child blocks",
		Example: "  affine-pp-cli canvas card inspect --workspace <workspace-id> --doc <doc-id> --card <card-id> --json",
		Annotations: map[string]string{
			"mcp:read-only":    "true",
			"pp:requires-tier": "affine-workspace-fixture",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return err
			}
			result, err := canvaswrite.InspectCard(cfg, opts)
			if err != nil {
				return err
			}
			return writeJSON(cmd.OutOrStdout(), result)
		},
	}
	cmd.Flags().StringVar(&opts.WorkspaceID, "workspace", "", "AFFiNE workspace ID")
	cmd.Flags().StringVar(&opts.DocID, "doc", "", "AFFiNE document ID")
	cmd.Flags().StringVar(&opts.CardID, "card", "", "Canvas note/card block ID")
	return cmd
}

func newCanvasCardSetImageCmd(flags *rootFlags) *cobra.Command {
	var opts canvaswrite.CardImageOptions
	cmd := &cobra.Command{
		Use:     "set-image",
		Short:   "Set a card image/logo without replacing the whole card",
		Example: "  affine-pp-cli canvas card set-image --workspace <workspace-id> --doc <doc-id> --card <card-id> --source-id <blob-id> --alt LightRAG --dry-run --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return writeJSON(cmd.OutOrStdout(), map[string]any{
					"dry_run": true,
					"doc_id":  opts.DocID,
					"card_id": opts.CardID,
					"source":  opts.SourceID,
				})
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return err
			}
			result, err := canvaswrite.SetCardImage(cfg, opts)
			if err != nil {
				return err
			}
			return writeJSON(cmd.OutOrStdout(), result)
		},
	}
	cmd.Flags().StringVar(&opts.WorkspaceID, "workspace", "", "AFFiNE workspace ID")
	cmd.Flags().StringVar(&opts.DocID, "doc", "", "AFFiNE document ID")
	cmd.Flags().StringVar(&opts.CardID, "card", "", "Canvas note/card block ID")
	cmd.Flags().StringVar(&opts.SourceID, "source-id", "", "Existing AFFiNE blob source ID")
	cmd.Flags().StringVar(&opts.Alt, "alt", "", "Fallback alt text to remove, e.g. LightRAG")
	cmd.Flags().IntVar(&opts.Width, "width", 120, "Image width")
	cmd.Flags().IntVar(&opts.Height, "height", 90, "Image height")
	return cmd
}

func newCanvasPlanCmd(flags *rootFlags) *cobra.Command {
	var specPath string
	var orientation string
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Create a deterministic card layout plan from a JSON spec",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		Example: "  affine-pp-cli canvas example > comunidade.json\n" +
			"  affine-pp-cli canvas plan --json\n" +
			"  affine-pp-cli canvas plan --spec comunidade.json --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			spec, err := readCanvasBuildSpecOrExample(specPath, cmd.InOrStdin())
			if err != nil {
				return err
			}
			if orientation != "" {
				spec.Orientation = orientation
			}
			plan := buildCanvasPlan(spec)
			return writeJSON(cmd.OutOrStdout(), plan)
		},
	}
	cmd.Flags().StringVar(&specPath, "spec", "", "JSON layout spec path; reads stdin when empty")
	cmd.Flags().StringVar(&orientation, "orientation", "", "Layout orientation: vertical or horizontal")
	return cmd
}

func newCanvasApplyCmd(flags *rootFlags) *cobra.Command {
	var planPath string
	var workspaceID string
	var docID string
	var backupDir string
	var apply bool
	var live bool
	var connectorsOnly bool
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply or dry-run canvas operations for a generated plan",
		Long:  "Apply or dry-run canvas operations for a generated plan. Live Y.js writes require --live or --apply plus --workspace, --doc, --backup-dir and --yes.",
		Example: "  affine-pp-cli canvas apply --dry-run --json\n" +
			"  affine-pp-cli canvas example | affine-pp-cli canvas plan | affine-pp-cli canvas apply --dry-run --json\n" +
			"  affine-pp-cli canvas apply --plan plan.json --dry-run --json\n" +
			"  affine-pp-cli canvas apply --plan transform.json --live --workspace <workspace-id> --doc <doc-id> --backup-dir ./backups --yes --json\n" +
			"  affine-pp-cli canvas apply --plan layout.json --live --workspace <workspace-id> --doc <doc-id> --backup-dir ./backups --yes --json",
		Annotations: map[string]string{
			"pp:requires-tier": "affine-workspace-fixture",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			liveApply := apply || live
			if !flags.dryRun && !liveApply {
				return fmt.Errorf("canvas apply requires --dry-run, --live or --apply")
			}
			payload, err := readCanvasApplyPayload(planPath, cmd.InOrStdin())
			if err != nil {
				return err
			}
			switch plan := payload.(type) {
			case canvaswrite.ConnectorLabelPlan:
				if liveApply && !flags.dryRun {
					if !flags.yes {
						return fmt.Errorf("confirmation required: pass --yes for live canvas apply")
					}
					if err := canvaswrite.ValidateConnectorLabelApply(plan, canvaswrite.ConnectorLabelApplyOptions{WorkspaceID: workspaceID, DocID: docID, BackupDir: backupDir}); err != nil {
						return err
					}
					cfg, err := config.Load(flags.configPath)
					if err != nil {
						return err
					}
					result, err := canvaswrite.ApplyConnectorLabelPlan(cfg, plan, canvaswrite.ConnectorLabelApplyOptions{WorkspaceID: workspaceID, DocID: docID, BackupDir: backupDir})
					if err != nil {
						return err
					}
					return writeJSON(cmd.OutOrStdout(), result)
				}
				return writeJSON(cmd.OutOrStdout(), map[string]any{
					"dry_run":               true,
					"plan_type":             plan.PlanType,
					"plan_id":               plan.PlanID,
					"affected_ids":          plan.AffectedIDs,
					"operations":            plan.Operations,
					"semantic_diff_preview": canvaswrite.ConnectorLabelDiffPreview(plan.Operations),
					"live_write_supported":  true,
					"live_write_requires":   []string{"--live", "--workspace", "--doc", "--backup-dir", "--yes"},
				})
			case canvaswrite.TransformPlan:
				if liveApply && !flags.dryRun {
					if !flags.yes {
						return fmt.Errorf("confirmation required: pass --yes for live canvas apply")
					}
					if err := canvaswrite.ValidateTransformApply(plan, canvaswrite.TransformApplyOptions{WorkspaceID: workspaceID, DocID: docID, BackupDir: backupDir}); err != nil {
						return err
					}
					cfg, err := config.Load(flags.configPath)
					if err != nil {
						return err
					}
					result, err := canvaswrite.ApplyTransformPlan(cfg, plan, canvaswrite.TransformApplyOptions{WorkspaceID: workspaceID, DocID: docID, BackupDir: backupDir})
					if err != nil {
						return err
					}
					return writeJSON(cmd.OutOrStdout(), result)
				}
				return writeJSON(cmd.OutOrStdout(), map[string]any{
					"dry_run":               true,
					"plan_type":             plan.PlanType,
					"plan_id":               plan.PlanID,
					"affected_ids":          plan.AffectedIDs,
					"operations":            plan.Operations,
					"semantic_diff_preview": canvaswrite.TransformDiffPreview(plan.Operations),
					"live_write_supported":  true,
					"live_write_requires":   []string{"--live", "--workspace", "--doc", "--backup-dir", "--yes"},
				})
			case canvasPlan:
				if connectorsOnly {
					var err error
					plan, err = canvasPlanConnectorsOnly(plan)
					if err != nil {
						return err
					}
				}
				if liveApply && !flags.dryRun {
					if !flags.yes {
						return fmt.Errorf("confirmation required: pass --yes for live canvas apply")
					}
					if err := validateCanvasLayoutApply(plan, workspaceID, docID, backupDir); err != nil {
						return err
					}
					cfg, err := config.Load(flags.configPath)
					if err != nil {
						return err
					}
					result, err := applyCanvasLayoutPlan(cfg, plan, workspaceID, docID, backupDir)
					if err != nil {
						return err
					}
					return writeJSON(cmd.OutOrStdout(), result)
				}
				if !flags.dryRun {
					return fmt.Errorf("canvas layout apply requires --dry-run or --live")
				}
				return writeJSON(cmd.OutOrStdout(), map[string]any{
					"dry_run":              true,
					"plan_type":            "canvas_layout",
					"frame":                plan.Frame,
					"live_write_supported": true,
					"live_write_requires":  []string{"--live", "--workspace", "--doc", "--backup-dir", "--yes"},
					"operations": map[string]any{
						"upsert_nodes":       plan.Nodes,
						"upsert_connections": plan.Connections,
					},
				})
			default:
				return fmt.Errorf("unsupported canvas apply payload")
			}
		},
	}
	cmd.Flags().StringVar(&planPath, "plan", "", "JSON plan path; reads stdin when empty")
	cmd.Flags().BoolVar(&apply, "apply", false, "Push a validated transform or layout plan to AFFiNE")
	cmd.Flags().BoolVar(&live, "live", false, "Alias for --apply; push a validated transform or layout plan to AFFiNE")
	cmd.Flags().StringVar(&workspaceID, "workspace", "", "AFFiNE workspace ID for live apply")
	cmd.Flags().StringVar(&docID, "doc", "", "AFFiNE document ID for live apply")
	cmd.Flags().StringVar(&backupDir, "backup-dir", "", "Directory for before/delta backups when applying")
	cmd.Flags().BoolVar(&connectorsOnly, "connectors-only", false, "For layout plans, refresh only connectors and preserve current card positions")
	return cmd
}

func canvasPlanConnectorsOnly(plan canvasPlan) (canvasPlan, error) {
	if len(plan.Connections) == 0 {
		return canvasPlan{}, fmt.Errorf("--connectors-only requires at least one connection")
	}
	plan.Nodes = nil
	return plan, nil
}

func validateCanvasLayoutApply(plan canvasPlan, workspaceID, docID, backupDir string) error {
	if workspaceID == "" {
		return fmt.Errorf("--workspace is required for live canvas apply")
	}
	if docID == "" {
		return fmt.Errorf("--doc is required for live canvas apply")
	}
	if backupDir == "" {
		return fmt.Errorf("--backup-dir is required for live canvas apply")
	}
	report := validateCanvasPlan(plan)
	if !report.OK {
		return fmt.Errorf("canvas layout plan is invalid: %d issue(s)", report.IssueCount)
	}
	return nil
}

func applyCanvasLayoutPlan(cfg *config.Config, plan canvasPlan, workspaceID, docID, backupDir string) (canvasLayoutApplyResult, error) {
	bearer := cfg.AffineToken
	if bearer == "" {
		bearer = strings.TrimPrefix(cfg.AuthHeader(), "Bearer ")
	}
	client, err := socketio.Connect(socketio.WSURLFromGraphQL(strings.TrimRight(cfg.BaseURL, "/")+"/graphql"), "", bearer)
	if err != nil {
		return canvasLayoutApplyResult{}, err
	}
	defer client.Close()
	if err := client.JoinWorkspace(workspaceID, "0.26.0"); err != nil {
		return canvasLayoutApplyResult{}, err
	}

	engine, err := yjs.NewEngine()
	if err != nil {
		return canvasLayoutApplyResult{}, err
	}
	loaded, err := client.LoadDoc(workspaceID, docID)
	if err != nil {
		return canvasLayoutApplyResult{}, err
	}
	if loaded.Missing == "" {
		return canvasLayoutApplyResult{}, fmt.Errorf("document returned empty snapshot")
	}
	doc, err := engine.ApplyBase64Update(loaded.Missing)
	if err != nil {
		return canvasLayoutApplyResult{}, err
	}
	blocks, err := engine.ReadBlocks(doc)
	if err != nil {
		return canvasLayoutApplyResult{}, err
	}
	before := canvaswrite.CheckBlocksIntegrity(docID, blocks)
	if !before.OK {
		return canvasLayoutApplyResult{}, fmt.Errorf("canvas doc integrity failed before layout apply")
	}
	if err := writeCanvasApplyBackup(backupDir, "before", loaded.Missing); err != nil {
		return canvasLayoutApplyResult{}, err
	}
	stateVector, err := engine.SaveStateVector(doc)
	if err != nil {
		return canvasLayoutApplyResult{}, err
	}
	var patch struct {
		UpsertedNodes      []string `json:"upserted_nodes"`
		UpsertedConnectors []string `json:"upserted_connectors"`
	}
	steps := canvasLayoutApplySteps(plan)
	proofFields := []string{"before.bin", "before.b64", "post_integrity"}
	for _, step := range steps {
		rawPatch, err := engine.RunScript(canvasLayoutApplyScript(doc, step.plan))
		if err != nil {
			return canvasLayoutApplyResult{}, fmt.Errorf("apply canvas layout %s: %w", step.name, err)
		}
		var stepPatch struct {
			UpsertedNodes      []string `json:"upserted_nodes"`
			UpsertedConnectors []string `json:"upserted_connectors"`
		}
		if err := json.Unmarshal([]byte(rawPatch), &stepPatch); err != nil {
			return canvasLayoutApplyResult{}, fmt.Errorf("parse layout apply result %s: %w", step.name, err)
		}
		patch.UpsertedNodes = append(patch.UpsertedNodes, stepPatch.UpsertedNodes...)
		patch.UpsertedConnectors = append(patch.UpsertedConnectors, stepPatch.UpsertedConnectors...)
		blocks, err = engine.ReadBlocks(doc)
		if err != nil {
			return canvasLayoutApplyResult{}, err
		}
		if err := canvaswrite.EnsureBlocksIntegrity(docID, blocks, "after local layout "+step.name); err != nil {
			return canvasLayoutApplyResult{}, err
		}
		delta, err := engine.EncodeDelta(doc, stateVector)
		if err != nil {
			return canvasLayoutApplyResult{}, err
		}
		if err := writeCanvasApplyBackup(backupDir, "delta-"+step.name, delta); err != nil {
			return canvasLayoutApplyResult{}, err
		}
		proofFields = append(proofFields, "delta-"+step.name+".bin", "delta-"+step.name+".b64")
		if err := client.PushDocUpdate(workspaceID, docID, delta); err != nil {
			return canvasLayoutApplyResult{}, err
		}
		stateVector, err = engine.SaveStateVector(doc)
		if err != nil {
			return canvasLayoutApplyResult{}, err
		}
	}
	reloaded, err := client.LoadDoc(workspaceID, docID)
	if err != nil {
		return canvasLayoutApplyResult{}, err
	}
	reloadedDoc, err := engine.ApplyBase64Update(reloaded.Missing)
	if err != nil {
		return canvasLayoutApplyResult{}, err
	}
	reloadedBlocks, err := engine.ReadBlocks(reloadedDoc)
	if err != nil {
		return canvasLayoutApplyResult{}, err
	}
	after := canvaswrite.CheckBlocksIntegrity(docID, reloadedBlocks)
	if !after.OK {
		return canvasLayoutApplyResult{}, fmt.Errorf("canvas doc integrity failed after pushed layout")
	}
	return canvasLayoutApplyResult{
		PlanType:           "canvas_layout",
		DocID:              docID,
		DryRun:             false,
		Applied:            true,
		BackupDir:          backupDir,
		UpsertedNodes:      patch.UpsertedNodes,
		UpsertedConnectors: patch.UpsertedConnectors,
		Before:             before,
		After:              after,
		Proof:              canvaswrite.TransformProof{Required: true, Fields: proofFields},
	}, nil
}

type canvasLayoutApplyStep struct {
	name string
	plan canvasPlan
}

func canvasLayoutApplySteps(plan canvasPlan) []canvasLayoutApplyStep {
	var steps []canvasLayoutApplyStep
	if len(plan.Nodes) > 0 {
		step := plan
		step.Connections = nil
		steps = append(steps, canvasLayoutApplyStep{name: "nodes", plan: step})
	}
	if len(plan.Connections) > 0 {
		step := plan
		step.Nodes = nil
		steps = append(steps, canvasLayoutApplyStep{name: "connectors", plan: step})
	}
	return steps
}

func canvasLayoutApplyScript(doc int, plan canvasPlan) string {
	rawPlan, _ := json.Marshal(plan)
	return fmt.Sprintf(`
		(function() {
			var doc = globalThis._docs[%d];
			var blocks = doc.getMap("blocks");
			var plan = %s;
			var pageId = "";
			blocks.forEach(function(block, id) {
				if (!pageId && block instanceof Y.Map && block.get("sys:flavour") === "affine:page") pageId = id;
			});
			if (!pageId) throw new Error("page root not found");
			var page = blocks.get(pageId);
			var pageChildren = page.get("sys:children");
			if (!(pageChildren instanceof Y.Array)) {
				pageChildren = new Y.Array();
				page.set("sys:children", pageChildren);
			}
			function hasChild(children, id) {
				for (var i = 0; i < children.length; i++) if (children.get(i) === id) return true;
				return false;
			}
			function ytext(value) {
				var t = new Y.Text();
				t.insert(0, String(value || ""));
				return t;
			}
			function xywh(n) {
				return "[" + [n.x || 0, n.y || 0, n.w || 360, n.h || 220].map(function(v) {
					return Number(v).toString();
				}).join(",") + "]";
			}
			var byName = {};
			var upsertedNodes = [];
			for (var i = 0; i < (plan.nodes || []).length; i++) {
				var n = plan.nodes[i];
				if (!n.id) throw new Error("node missing id");
				var note = blocks.get(n.id);
				var createdNote = false;
				if (!(note instanceof Y.Map)) {
					note = new Y.Map();
					note.set("sys:id", n.id);
					note.set("sys:flavour", "affine:note");
					note.set("sys:version", 1);
					note.set("sys:children", new Y.Array());
					note.set("prop:displayMode", n.display_mode || "edgeless");
					note.set("prop:hidden", false);
					note.set("prop:index", "a0");
					blocks.set(n.id, note);
					createdNote = true;
				}
				note.set("prop:xywh", xywh(n));
				if (n.display_mode) note.set("prop:displayMode", n.display_mode);
				if (n.background) note.set("prop:background", n.background);
				if (n.edgeless) note.set("prop:edgeless", n.edgeless);
				var children = note.get("sys:children");
				if (!(children instanceof Y.Array)) {
					children = new Y.Array();
					note.set("sys:children", children);
				}
				var textBlocks = Array.isArray(n.text_blocks) ? n.text_blocks : [];
				if (textBlocks.length > 0) {
					if (n.replace_children) {
						while (children.length > 0) children.delete(0, children.length);
					}
					for (var tb = 0; tb < textBlocks.length; tb++) {
						var spec = textBlocks[tb] || {};
						var richTextId = spec.id || (n.id + "-text-" + tb);
						var richTextBlock = blocks.get(richTextId);
						if (!(richTextBlock instanceof Y.Map)) {
							richTextBlock = new Y.Map();
							richTextBlock.set("sys:id", richTextId);
							richTextBlock.set("sys:flavour", "affine:paragraph");
							richTextBlock.set("sys:version", 1);
							richTextBlock.set("sys:type", spec.type || "text");
							richTextBlock.set("prop:type", spec.type || "text");
							richTextBlock.set("sys:children", new Y.Array());
							blocks.set(richTextId, richTextBlock);
						}
						richTextBlock.set("prop:text", ytext(spec.text || ""));
						if (!hasChild(children, richTextId)) children.insert(children.length, [richTextId]);
					}
				} else if (createdNote || n.text) {
					var textId = n.id + "-text";
					var textBlock = blocks.get(textId);
					if (!(textBlock instanceof Y.Map)) {
						textBlock = new Y.Map();
						textBlock.set("sys:id", textId);
						textBlock.set("sys:flavour", "affine:paragraph");
						textBlock.set("sys:version", 1);
						textBlock.set("sys:type", "text");
						textBlock.set("prop:type", "text");
						textBlock.set("sys:children", new Y.Array());
						blocks.set(textId, textBlock);
					}
					textBlock.set("prop:text", ytext(n.text || n.id));
					if (!hasChild(children, textId)) children.insert(children.length, [textId]);
				}
				if (!hasChild(pageChildren, n.id)) pageChildren.push([n.id]);
				byName[n.id] = n.id;
				if (n.text) byName[n.text] = n.id;
				upsertedNodes.push(n.id);
			}
			function endpoint(value) {
				return byName[value] || value;
			}
			function blockCenter(id) {
				function parse(id) {
					var b = blocks.get(id);
					var s = b && b.get("prop:xywh");
					var parts = String(s || "0,0,1,1").replace(/[\[\]]/g, "").split(",");
					return parts.map(function(p) { return Number(p) || 0; });
				}
				var xywh = parse(id);
				return {x: xywh[0] + xywh[2] / 2, y: xywh[1] + xywh[3] / 2};
			}
			function connectorAnchors(from, to) {
				var a = blockCenter(from), b = blockCenter(to);
				if (Math.abs(b.y - a.y) > 80) {
					return b.y >= a.y
						? {source: [0.5, 1], target: [0.5, 0]}
						: {source: [0.5, 0], target: [0.5, 1]};
				}
				return b.x >= a.x
					? {source: [1, 0.5], target: [0, 0.5]}
					: {source: [0, 0.5], target: [1, 0.5]};
			}
			function connectorID(from, to) {
				var s = from + ">" + to;
				var h = 2166136261;
				for (var i = 0; i < s.length; i++) {
					h ^= s.charCodeAt(i);
					h = Math.imul(h, 16777619);
				}
				return "c" + (h >>> 0).toString(36).padStart(9, "0").slice(0, 9);
			}
			function connectorIndex(i) {
				return "av" + String(i).padStart(4, "0");
			}
			function surfaceElements() {
				var surface = null;
				blocks.forEach(function(block) {
					if (!surface && block instanceof Y.Map && block.get("sys:flavour") === "affine:surface") surface = block;
				});
				if (!(surface instanceof Y.Map)) throw new Error("surface block not found");
				var raw = surface.get("prop:elements");
				var boxed = raw instanceof Y.Map && raw.get("type") === "$blocksuite:internal:native$";
				var value = boxed ? raw.get("value") : raw;
				var valueMap = value instanceof Y.Map && boxed ? value : new Y.Map();
				if (value instanceof Y.Map && !boxed) {
					value.forEach(function(existing, k) { valueMap.set(k, existing); });
				}
				if (!(value instanceof Y.Map) && value && typeof value === "object") {
					var plain = value.type === "$blocksuite:internal:native$" ? value.value : value;
					for (var k in plain) {
						if (k.charAt(0) === "_") continue;
						var existing = plain[k];
						if (existing instanceof Y.Map) {
							valueMap.set(k, existing);
						} else if (existing && typeof existing === "object") {
							var elementMap = new Y.Map();
							for (var prop in existing) elementMap.set(prop, existing[prop]);
							valueMap.set(k, elementMap);
						}
					}
				}
				return {surface: surface, value: valueMap, boxed: boxed};
			}
			var surface = null;
			var upsertedConnectors = [];
			for (var c = 0; c < (plan.connections || []).length; c++) {
				if (!surface) surface = surfaceElements();
				var edge = plan.connections[c];
				var from = endpoint(edge.from);
				var to = endpoint(edge.to);
				if (!(blocks.get(from) instanceof Y.Map)) throw new Error("missing connector source: " + edge.from);
				if (!(blocks.get(to) instanceof Y.Map)) throw new Error("missing connector target: " + edge.to);
				var id = connectorID(from, to);
				var anchors = connectorAnchors(from, to);
				var element = new Y.Map();
				element.set("id", id);
				element.set("type", "connector");
				element.set("index", connectorIndex(c));
				element.set("mode", 2);
				element.set("source", {id: from, position: anchors.source});
				element.set("target", {id: to, position: anchors.target});
				element.set("stroke", "#929292");
				element.set("strokeStyle", "solid");
				element.set("strokeWidth", 2);
				element.set("frontEndpointStyle", "None");
				element.set("rearEndpointStyle", "Arrow");
				element.set("rough", false);
				element.set("roughness", 1.4);
				element.set("seed", Math.floor(Math.random() * 2147483647));
				element.set("labelStyle", {
					color: {dark: "#ffffff", light: "#000000"},
					fontFamily: "blocksuite:surface:Inter",
					fontSize: 16,
					fontStyle: "normal",
					fontWeight: "400",
					textAlign: "center"
				});
				surface.value.set(id, element);
				upsertedConnectors.push(id);
			}
			if (surface) {
				if (!surface.boxed) {
					var boxed = new Y.Map();
					boxed.set("type", "$blocksuite:internal:native$");
					boxed.set("value", surface.value);
					surface.surface.set("prop:elements", boxed);
				}
			}
			return JSON.stringify({upserted_nodes: upsertedNodes, upserted_connectors: upsertedConnectors});
		})()
	`, doc, string(rawPlan))
}

func writeCanvasApplyBackup(dir, name, b64 string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, name+".bin"), raw, 0o600); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, name+".b64"), []byte(b64), 0o600)
}

func newCanvasModelCmd(flags *rootFlags) *cobra.Command {
	var inputPath string
	cmd := &cobra.Command{
		Use:   "model",
		Short: "Normalize inspect-canvas JSON into nodes and connections",
		Example: "  affine-pp-cli canvas model --json\n" +
			"  affine-pp-cli canvas model --input inspect-canvas.json --json\n" +
			"  cat inspect-canvas.json | affine-pp-cli canvas model --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			model, err := readCanvasModelOrExample(inputPath, cmd.InOrStdin())
			if err != nil {
				return err
			}
			return writeJSON(cmd.OutOrStdout(), model)
		},
	}
	cmd.Flags().StringVar(&inputPath, "input", "", "inspect-canvas JSON path; reads stdin when empty")
	return cmd
}

func newCanvasValidateCmd(flags *rootFlags) *cobra.Command {
	var planPath string
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate a canvas plan before applying it",
		Example: "  affine-pp-cli canvas validate --json\n" +
			"  affine-pp-cli canvas example | affine-pp-cli canvas plan | affine-pp-cli canvas validate --json\n" +
			"  affine-pp-cli canvas validate --plan plan.json --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			plan, err := readCanvasPlanOrExample(planPath, cmd.InOrStdin())
			if err != nil {
				return err
			}
			return writeJSON(cmd.OutOrStdout(), validateCanvasPlan(plan))
		},
	}
	cmd.Flags().StringVar(&planPath, "plan", "", "JSON plan path; reads stdin when empty")
	return cmd
}

func newCanvasExampleCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "example",
		Short: "Print a vertical tree JSON spec",
		Example: "  affine-pp-cli canvas example --json\n" +
			"  affine-pp-cli canvas example > comunidade.json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return writeJSON(cmd.OutOrStdout(), exampleCanvasBuildSpec())
		},
	}
}

func readCanvasBuildSpecOrExample(path string, stdin io.Reader) (canvasBuildSpec, error) {
	if path != "" {
		return readCanvasBuildSpec(path, stdin)
	}
	data, err := io.ReadAll(stdin)
	if err != nil {
		return canvasBuildSpec{}, fmt.Errorf("reading stdin: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return exampleCanvasBuildSpec(), nil
	}
	var spec canvasBuildSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return spec, fmt.Errorf("parsing canvas spec JSON: %w", err)
	}
	return spec, nil
}

func exampleCanvasBuildSpec() canvasBuildSpec {
	return canvasBuildSpec{
		Frame:       "COMUNIDADE",
		Orientation: "vertical",
		Hub:         "Carta Clube",
		Levels: [][]string{
			{"Como Trabalhamos", "Identidade Visual", "Carta Clube - Principios", "Carta Clube - Qualidades"},
			{"Carta Clube"},
			{"Coding", "Comunidade", "Criacao", "Execucao"},
		},
		Connections: []canvasConnection{
			{From: "Carta Clube - Principios", To: "Carta Clube"},
			{From: "Carta Clube", To: "Coding"},
			{From: "Carta Clube", To: "Comunidade"},
			{From: "Carta Clube", To: "Criacao"},
			{From: "Carta Clube", To: "Execucao"},
		},
	}
}

func readCanvasBuildSpec(path string, stdin io.Reader) (canvasBuildSpec, error) {
	var spec canvasBuildSpec
	data, err := readAllOrFile(path, stdin)
	if err != nil {
		return spec, err
	}
	if err := json.Unmarshal(data, &spec); err != nil {
		return spec, fmt.Errorf("parsing canvas spec JSON: %w", err)
	}
	return spec, nil
}

func readCanvasPlan(path string, stdin io.Reader) (canvasPlan, error) {
	var plan canvasPlan
	data, err := readAllOrFile(path, stdin)
	if err != nil {
		return plan, err
	}
	if err := json.Unmarshal(data, &plan); err != nil {
		return plan, fmt.Errorf("parsing canvas plan JSON: %w", err)
	}
	return plan, nil
}

func readCanvasSelectors(path string, stdin io.Reader, allowEmpty bool) (canvaswrite.SearchResult, error) {
	if path == "" {
		if allowEmpty {
			return canvaswrite.SearchResult{SourceMode: "direct"}, nil
		}
		return canvaswrite.SearchResult{}, fmt.Errorf("pass --selectors or --id")
	}
	var data []byte
	var err error
	if path == "-" {
		data, err = io.ReadAll(stdin)
		if err != nil {
			return canvaswrite.SearchResult{}, fmt.Errorf("reading stdin: %w", err)
		}
	} else {
		data, err = os.ReadFile(path)
		if err != nil {
			return canvaswrite.SearchResult{}, err
		}
	}
	var selectors canvaswrite.SearchResult
	if err := json.Unmarshal(data, &selectors); err != nil {
		return selectors, fmt.Errorf("parsing canvas search JSON: %w", err)
	}
	return selectors, nil
}

func readCanvasApplyPayload(path string, stdin io.Reader) (any, error) {
	data, err := readAllOrFile(path, stdin)
	if err != nil {
		if path == "" {
			return buildCanvasPlan(exampleCanvasBuildSpec()), nil
		}
		return nil, err
	}
	var envelope struct {
		PlanType string `json:"plan_type"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("parsing canvas apply JSON: %w", err)
	}
	if envelope.PlanType == "canvas_transform" {
		var plan canvaswrite.TransformPlan
		if err := json.Unmarshal(data, &plan); err != nil {
			return nil, fmt.Errorf("parsing canvas transform plan JSON: %w", err)
		}
		return plan, nil
	}
	if envelope.PlanType == "canvas_connector_labels" {
		var plan canvaswrite.ConnectorLabelPlan
		if err := json.Unmarshal(data, &plan); err != nil {
			return nil, fmt.Errorf("parsing canvas connector label plan JSON: %w", err)
		}
		return plan, nil
	}
	var plan canvasPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("parsing canvas plan JSON: %w", err)
	}
	return plan, nil
}

func readCanvasPlanOrExample(path string, stdin io.Reader) (canvasPlan, error) {
	if path != "" {
		return readCanvasPlan(path, stdin)
	}
	data, err := io.ReadAll(stdin)
	if err != nil {
		return canvasPlan{}, fmt.Errorf("reading stdin: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return buildCanvasPlan(exampleCanvasBuildSpec()), nil
	}
	var plan canvasPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return plan, fmt.Errorf("parsing canvas plan JSON: %w", err)
	}
	return plan, nil
}

func readCanvasModel(path string, stdin io.Reader) (canvasModel, error) {
	data, err := readAllOrFile(path, stdin)
	if err != nil {
		return canvasModel{}, err
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return canvasModel{}, fmt.Errorf("parsing inspect-canvas JSON: %w", err)
	}
	var model canvasModel
	walkCanvasValue(raw, &model)
	return model, nil
}

func readCanvasModelOrExample(path string, stdin io.Reader) (canvasModel, error) {
	if path != "" {
		return readCanvasModel(path, stdin)
	}
	data, err := io.ReadAll(stdin)
	if err != nil {
		return canvasModel{}, fmt.Errorf("reading stdin: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return exampleCanvasModel(), nil
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return canvasModel{}, fmt.Errorf("parsing inspect-canvas JSON: %w", err)
	}
	var model canvasModel
	walkCanvasValue(raw, &model)
	return model, nil
}

func exampleCanvasModel() canvasModel {
	return canvasModel{
		Nodes: []map[string]any{
			{"id": "carta-clube", "text": "Carta Clube", "xywh": "[0,0,360,220]"},
			{"id": "comunidade", "text": "Comunidade", "xywh": "[0,360,360,220]"},
		},
		Connections: []map[string]any{
			{"id": "carta-clube-comunidade", "source": "carta-clube", "target": "comunidade", "type": "connector"},
		},
	}
}

func walkCanvasValue(v any, model *canvasModel) {
	switch x := v.(type) {
	case map[string]any:
		if connection, ok := normalizeCanvasConnection(x); ok {
			model.Connections = append(model.Connections, connection)
		} else if node, ok := normalizeCanvasNode(x); ok {
			model.Nodes = append(model.Nodes, node)
		}
		for key, child := range x {
			if key == "props" {
				continue
			}
			walkCanvasValue(child, model)
		}
	case []any:
		for _, child := range x {
			walkCanvasValue(child, model)
		}
	}
}

func normalizeCanvasNode(src map[string]any) (map[string]any, bool) {
	text := firstCanvasString(src, "text", "title")
	xywh := firstCanvasString(src, "xywh", "prop:xywh")
	if props, _ := src["props"].(map[string]any); props != nil {
		if text == "" {
			text = firstCanvasString(props, "text", "title")
		}
		if xywh == "" {
			xywh = firstCanvasString(props, "xywh", "prop:xywh")
		}
	}
	if text == "" || xywh == "" {
		return nil, false
	}
	out := compactCanvasMap(src, "id", "flavour", "type", "text", "title", "surface_id")
	out["text"] = text
	out["xywh"] = xywh
	return out, true
}

func normalizeCanvasConnection(src map[string]any) (map[string]any, bool) {
	source := firstCanvasString(src, "source", "from")
	target := firstCanvasString(src, "target", "to")
	props, _ := src["props"].(map[string]any)
	if props != nil {
		if source == "" {
			source = firstCanvasString(props, "source", "from")
		}
		if target == "" {
			target = firstCanvasString(props, "target", "to")
		}
	}
	if source == "" || target == "" {
		return nil, false
	}
	out := compactCanvasMap(src, "id", "type", "flavour", "text", "surface_id", "xywh", "prop:xywh")
	if props != nil {
		if _, ok := out["type"]; !ok {
			copyCanvasField(out, props, "type")
		}
		if _, ok := out["xywh"]; !ok {
			copyCanvasField(out, props, "xywh")
		}
	}
	out["source"] = source
	out["target"] = target
	return out, true
}

func firstCanvasString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := canvasStringValue(m[key]); value != "" {
			return value
		}
	}
	return ""
}

func canvasStringValue(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case map[string]any:
		return firstCanvasString(x, "id", "blockId", "elementId")
	default:
		if x == nil {
			return ""
		}
		return strings.TrimSpace(fmt.Sprint(x))
	}
}

func copyCanvasField(dst map[string]any, src map[string]any, key string) {
	if value, ok := src[key]; ok && value != nil && value != "" {
		dst[key] = value
	}
}

func compactCanvasMap(src map[string]any, keys ...string) map[string]any {
	out := map[string]any{}
	for _, key := range keys {
		if value, ok := src[key]; ok && value != nil && value != "" {
			out[key] = value
		}
	}
	return out
}

func buildCanvasPlan(spec canvasBuildSpec) canvasPlan {
	spec = normalizeCanvasSpec(spec)
	var nodes []canvasNode
	seen := map[string]bool{}
	for level, names := range spec.Levels {
		for idx, name := range names {
			node := canvasNode{
				ID:    slugCanvasID(name),
				Text:  name,
				Role:  canvasRole(name, spec.Hub),
				Level: level,
				W:     spec.Card.W,
				H:     spec.Card.H,
			}
			node.X, node.Y = canvasPosition(spec, level, idx, len(names))
			if seen[node.ID] {
				node.ID = fmt.Sprintf("%s-%d-%d", node.ID, level, idx)
			}
			seen[node.ID] = true
			nodes = append(nodes, node)
		}
	}
	connections := spec.Connections
	if len(connections) == 0 && spec.Hub != "" {
		connections = inferredHubConnections(spec)
	}
	return canvasPlan{
		Frame:       spec.Frame,
		Orientation: spec.Orientation,
		Nodes:       nodes,
		Connections: connections,
		Warnings:    canvasPlanWarnings(spec),
	}
}

func normalizeCanvasSpec(spec canvasBuildSpec) canvasBuildSpec {
	if spec.Orientation == "" {
		spec.Orientation = "vertical"
	}
	spec.Orientation = strings.ToLower(spec.Orientation)
	if spec.Card.W == 0 {
		spec.Card.W = 360
	}
	if spec.Card.H == 0 {
		spec.Card.H = 220
	}
	if spec.Spacing.X == 0 {
		spec.Spacing.X = 80
	}
	if spec.Spacing.Y == 0 {
		spec.Spacing.Y = 140
	}
	return spec
}

func canvasPosition(spec canvasBuildSpec, level, idx, count int) (float64, float64) {
	span := float64(count-1) * (spec.Card.W + spec.Spacing.X)
	if spec.Orientation == "horizontal" {
		return spec.Origin.X + float64(level)*(spec.Card.W+spec.Spacing.X), spec.Origin.Y + float64(idx)*(spec.Card.H+spec.Spacing.Y) - span/2
	}
	return spec.Origin.X + float64(idx)*(spec.Card.W+spec.Spacing.X) - span/2, spec.Origin.Y + float64(level)*(spec.Card.H+spec.Spacing.Y)
}

func inferredHubConnections(spec canvasBuildSpec) []canvasConnection {
	var out []canvasConnection
	for _, level := range spec.Levels {
		for _, name := range level {
			if name != spec.Hub {
				out = append(out, canvasConnection{From: spec.Hub, To: name, Type: "tree"})
			}
		}
	}
	return out
}

func canvasPlanWarnings(spec canvasBuildSpec) []string {
	var warnings []string
	if spec.Orientation != "vertical" && spec.Orientation != "horizontal" {
		warnings = append(warnings, "unknown orientation; use vertical or horizontal")
	}
	if spec.Hub == "" {
		warnings = append(warnings, "hub is empty; connections must be explicit")
	}
	if len(spec.Levels) == 0 {
		warnings = append(warnings, "levels is empty")
	}
	return warnings
}

func validateCanvasPlan(plan canvasPlan) canvasValidationReport {
	var issues []canvasValidationIssue
	if plan.Frame == "" {
		issues = append(issues, canvasValidationIssue{
			Severity: "warning",
			Code:     "missing_frame",
			Message:  "plan frame is empty",
		})
	}
	hasNodes := len(plan.Nodes) > 0
	if hasNodes && plan.Orientation != "vertical" && plan.Orientation != "horizontal" {
		issues = append(issues, canvasValidationIssue{
			Severity: "error",
			Code:     "unknown_orientation",
			Message:  "orientation must be vertical or horizontal",
		})
	}
	if !hasNodes && len(plan.Connections) == 0 {
		issues = append(issues, canvasValidationIssue{
			Severity: "error",
			Code:     "empty_plan",
			Message:  "plan has no nodes or connections",
		})
	}

	known := map[string]bool{}
	for _, node := range plan.Nodes {
		if node.ID == "" {
			issues = append(issues, canvasValidationIssue{
				Severity: "error",
				Code:     "missing_node_id",
				Message:  "node has empty id",
			})
			continue
		}
		if known[node.ID] {
			issues = append(issues, canvasValidationIssue{
				Severity: "error",
				Code:     "duplicate_node_id",
				Message:  "node id is duplicated",
				NodeID:   node.ID,
			})
		}
		known[node.ID] = true
		if node.Text != "" {
			known[node.Text] = true
		}
		if node.W <= 0 || node.H <= 0 {
			issues = append(issues, canvasValidationIssue{
				Severity: "error",
				Code:     "invalid_node_size",
				Message:  "node width and height must be positive",
				NodeID:   node.ID,
			})
		}
	}

	for _, conn := range plan.Connections {
		if conn.From == "" || conn.To == "" {
			issues = append(issues, canvasValidationIssue{
				Severity: "error",
				Code:     "invalid_connection",
				Message:  "connection must include from and to",
				From:     conn.From,
				To:       conn.To,
			})
			continue
		}
		if hasNodes && (!known[conn.From] || !known[conn.To]) {
			issues = append(issues, canvasValidationIssue{
				Severity: "error",
				Code:     "orphan_connection",
				Message:  "connection endpoint is not present in plan nodes",
				From:     conn.From,
				To:       conn.To,
			})
		}
	}

	for i := 0; i < len(plan.Nodes); i++ {
		for j := i + 1; j < len(plan.Nodes); j++ {
			if canvasNodesOverlap(plan.Nodes[i], plan.Nodes[j]) {
				issues = append(issues, canvasValidationIssue{
					Severity: "warning",
					Code:     "overlapping_nodes",
					Message:  fmt.Sprintf("nodes %q and %q overlap", plan.Nodes[i].ID, plan.Nodes[j].ID),
					NodeID:   plan.Nodes[i].ID,
				})
			}
		}
	}

	ok := true
	for _, issue := range issues {
		if issue.Severity == "error" {
			ok = false
			break
		}
	}
	return canvasValidationReport{
		OK:         ok,
		IssueCount: len(issues),
		Issues:     issues,
	}
}

func canvasNodesOverlap(a, b canvasNode) bool {
	if a.W <= 0 || a.H <= 0 || b.W <= 0 || b.H <= 0 {
		return false
	}
	return a.X < b.X+b.W && a.X+a.W > b.X && a.Y < b.Y+b.H && a.Y+a.H > b.Y
}

func canvasRole(name, hub string) string {
	if name == hub {
		return "hub"
	}
	return "card"
}

func slugCanvasID(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if ok {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func splitCanvasKeywords(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func readAllOrFile(path string, stdin io.Reader) ([]byte, error) {
	if path != "" && path != "-" {
		return os.ReadFile(path)
	}
	data, err := io.ReadAll(stdin)
	if err != nil {
		return nil, fmt.Errorf("reading stdin: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, fmt.Errorf("empty input; pass --spec/--plan/--input or pipe JSON on stdin")
	}
	return data, nil
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
