package canvaswrite

import (
	"encoding/json"
	"fmt"
	"strings"

	"affine-pp-cli/internal/config"
	"affine-pp-cli/internal/yjs"
)

type TransformApplyOptions struct {
	WorkspaceID string
	DocID       string
	BackupDir   string
}

type TransformApplyResult struct {
	PlanType            string               `json:"plan_type"`
	PlanID              string               `json:"plan_id"`
	DocID               string               `json:"doc_id"`
	DryRun              bool                 `json:"dry_run"`
	Applied             bool                 `json:"applied"`
	BackupDir           string               `json:"backup_dir"`
	AffectedIDs         []string             `json:"affected_ids"`
	Operations          []TransformOperation `json:"operations"`
	SemanticDiffPreview []DiffIssue          `json:"semantic_diff_preview"`
	Before              DocIntegrityResult   `json:"before"`
	After               DocIntegrityResult   `json:"after"`
	Proof               TransformProof       `json:"proof"`
}

func ApplyTransformPlan(cfg *config.Config, plan TransformPlan, opts TransformApplyOptions) (TransformApplyResult, error) {
	if err := ValidateTransformApply(plan, opts); err != nil {
		return TransformApplyResult{}, err
	}
	client, err := connect(cfg, opts.WorkspaceID)
	if err != nil {
		return TransformApplyResult{}, err
	}
	defer client.Close()

	engine, err := yjs.NewEngine()
	if err != nil {
		return TransformApplyResult{}, err
	}
	loaded, err := client.LoadDoc(opts.WorkspaceID, opts.DocID)
	if err != nil {
		return TransformApplyResult{}, err
	}
	if loaded.Missing == "" {
		return TransformApplyResult{}, fmt.Errorf("document returned empty snapshot")
	}
	doc, err := engine.ApplyBase64Update(loaded.Missing)
	if err != nil {
		return TransformApplyResult{}, err
	}
	blocks, err := engine.ReadBlocks(doc)
	if err != nil {
		return TransformApplyResult{}, err
	}
	before := CheckBlocksIntegrity(opts.DocID, blocks)
	if !before.OK {
		return TransformApplyResult{}, fmt.Errorf("canvas doc integrity failed before transform apply: %s", integritySummary(before))
	}
	if err := writeRepairBackup(opts.BackupDir, "before", loaded.Missing); err != nil {
		return TransformApplyResult{}, err
	}
	stateVector, err := engine.SaveStateVector(doc)
	if err != nil {
		return TransformApplyResult{}, err
	}
	if _, err := engine.RunScript(transformApplyScript(doc, plan.Operations)); err != nil {
		return TransformApplyResult{}, fmt.Errorf("apply transform operations: %w", err)
	}
	blocks, err = engine.ReadBlocks(doc)
	if err != nil {
		return TransformApplyResult{}, err
	}
	if err := EnsureBlocksIntegrity(opts.DocID, blocks, "after local transform apply"); err != nil {
		return TransformApplyResult{}, err
	}
	delta, err := engine.EncodeDelta(doc, stateVector)
	if err != nil {
		return TransformApplyResult{}, err
	}
	if err := writeRepairBackup(opts.BackupDir, "delta", delta); err != nil {
		return TransformApplyResult{}, err
	}
	if err := client.PushDocUpdate(opts.WorkspaceID, opts.DocID, delta); err != nil {
		return TransformApplyResult{}, err
	}
	reloaded, err := client.LoadDoc(opts.WorkspaceID, opts.DocID)
	if err != nil {
		return TransformApplyResult{}, err
	}
	reloadedDoc, err := engine.ApplyBase64Update(reloaded.Missing)
	if err != nil {
		return TransformApplyResult{}, err
	}
	reloadedBlocks, err := engine.ReadBlocks(reloadedDoc)
	if err != nil {
		return TransformApplyResult{}, err
	}
	after := CheckBlocksIntegrity(opts.DocID, reloadedBlocks)
	if !after.OK {
		return TransformApplyResult{}, fmt.Errorf("canvas doc integrity failed after pushed transform: %s", integritySummary(after))
	}
	return TransformApplyResult{
		PlanType:            plan.PlanType,
		PlanID:              plan.PlanID,
		DocID:               opts.DocID,
		DryRun:              false,
		Applied:             true,
		BackupDir:           opts.BackupDir,
		AffectedIDs:         plan.AffectedIDs,
		Operations:          plan.Operations,
		SemanticDiffPreview: TransformDiffPreview(plan.Operations),
		Before:              before,
		After:               after,
		Proof:               TransformProof{Required: true, Fields: []string{"before.bin", "before.b64", "delta.bin", "delta.b64", "post_integrity"}},
	}, nil
}

func ValidateTransformApply(plan TransformPlan, opts TransformApplyOptions) error {
	if plan.PlanType != "canvas_transform" {
		return fmt.Errorf("live canvas apply requires plan_type canvas_transform")
	}
	if strings.TrimSpace(plan.PlanID) == "" {
		return fmt.Errorf("canvas transform plan requires plan_id")
	}
	if len(plan.Operations) == 0 {
		return fmt.Errorf("canvas transform plan requires operations")
	}
	if err := validateTransformOperations(plan.Operations); err != nil {
		return err
	}
	if !plan.Integrity.OK {
		return fmt.Errorf("canvas transform plan integrity is not OK")
	}
	if opts.WorkspaceID == "" {
		return fmt.Errorf("--workspace is required for live canvas apply")
	}
	if opts.DocID == "" {
		return fmt.Errorf("--doc is required for live canvas apply")
	}
	if plan.DocID != "" && plan.DocID != opts.DocID {
		return fmt.Errorf("plan doc_id %q does not match --doc %q", plan.DocID, opts.DocID)
	}
	if opts.BackupDir == "" {
		return fmt.Errorf("--backup-dir is required for live canvas apply")
	}
	return nil
}

func validateTransformOperations(ops []TransformOperation) error {
	for _, op := range ops {
		if strings.TrimSpace(op.ID) == "" {
			return fmt.Errorf("canvas transform operation requires id")
		}
		switch {
		case op.Kind == "move" || op.Kind == "resize" || strings.HasPrefix(op.Kind, "align_") || strings.HasPrefix(op.Kind, "distribute_"):
			xywh, ok := transformAfterXYWH(op.After)
			if !ok {
				return fmt.Errorf("%s operation for %q requires after xywh", op.Kind, op.ID)
			}
			if xywh[2] <= 0 || xywh[3] <= 0 {
				return fmt.Errorf("%s operation for %q has invalid size", op.Kind, op.ID)
			}
		case op.Kind == "set_display_mode":
			if value, ok := op.After.(string); !ok || strings.TrimSpace(value) == "" {
				return fmt.Errorf("set_display_mode operation for %q requires after string", op.ID)
			}
		case op.Kind == CardCreateKind:
			// Per-operation invariants are checked as a group below, so block IDs
			// duplicated across cards in the same plan are rejected too.
		case op.Kind == "set_metadata":
			metadata, ok := transformAfterMetadata(op.After)
			if !ok || len(metadata) != 1 {
				return fmt.Errorf("set_metadata operation for %q requires one metadata key", op.ID)
			}
			for key := range metadata {
				key = strings.TrimSpace(key)
				if key == "" || strings.HasPrefix(key, "sys:") {
					return fmt.Errorf("set_metadata operation for %q has unsafe metadata key %q", op.ID, key)
				}
			}
		default:
			return fmt.Errorf("unsupported transform operation %q", op.Kind)
		}
	}
	return validateCardCreateOperations(ops)
}

func TransformDiffPreview(ops []TransformOperation) []DiffIssue {
	issues := make([]DiffIssue, 0, len(ops))
	for _, op := range ops {
		issue := DiffIssue{
			ID:                  op.ID,
			Before:              op.Before,
			After:               op.After,
			Severity:            "info",
			SuggestedNextAction: "Verify this planned transform before live apply.",
		}
		switch {
		case op.Kind == "move" || op.Kind == "resize" || strings.HasPrefix(op.Kind, "align_") || strings.HasPrefix(op.Kind, "distribute_"):
			issue.Category = "geometry_changed"
		case op.Kind == "set_display_mode":
			issue.Category = "display_mode_changed"
		case op.Kind == CardCreateKind:
			issue.Category = "card_created"
			issue.SuggestedNextAction = "Review card geometry, frame and text before live apply."
		case op.Kind == "set_metadata":
			issue.Category = "metadata_changed"
		default:
			issue.Category = op.Kind
			issue.Severity = "warning"
		}
		issues = append(issues, issue)
	}
	return issues
}

func transformAfterXYWH(v any) ([]float64, bool) {
	switch x := v.(type) {
	case []float64:
		if len(x) != 4 {
			return nil, false
		}
		return x, true
	case []any:
		if len(x) != 4 {
			return nil, false
		}
		out := make([]float64, 4)
		for i, raw := range x {
			num, ok := raw.(float64)
			if !ok {
				return nil, false
			}
			out[i] = num
		}
		return out, true
	default:
		return nil, false
	}
}

func transformAfterMetadata(v any) (map[string]any, bool) {
	switch x := v.(type) {
	case map[string]any:
		return x, true
	case map[string]string:
		out := make(map[string]any, len(x))
		for key, value := range x {
			out[key] = value
		}
		return out, true
	default:
		return nil, false
	}
}

func transformApplyScript(doc int, ops []TransformOperation) string {
	rawOps, err := asciiJSON(ops)
	if err != nil {
		raw, _ := json.Marshal(ops)
		rawOps = string(raw)
	}
	return fmt.Sprintf(`
		(function() {
			var doc = globalThis._docs[%d];
			var blocks = doc.getMap("blocks");
			var ops = %s;
			function requireBlock(id) {
				var block = blocks.get(id);
				if (!(block instanceof Y.Map)) throw new Error("block not found: " + id);
				return block;
			}
			function xywhString(value) {
				if (!Array.isArray(value) || value.length !== 4) throw new Error("invalid xywh operation value");
				return "[" + value.map(function(v) {
					if (typeof v !== "number" || !isFinite(v)) throw new Error("invalid xywh number");
					return Number(v.toFixed(4)).toString();
				}).join(",") + "]";
			}
			%s
			var applied = [];
			for (var i = 0; i < ops.length; i++) {
				var op = ops[i];
				if (op.kind === "create_card") {
					createCard(op.id, op.after);
					applied.push(op.id);
					continue;
				}
				var block = requireBlock(op.id);
				if (op.kind === "move" || op.kind === "resize" || op.kind.indexOf("align_") === 0 || op.kind.indexOf("distribute_") === 0) {
					block.set("prop:xywh", xywhString(op.after));
				} else if (op.kind === "set_display_mode") {
					if (typeof op.after !== "string" || op.after.length === 0) throw new Error("invalid display mode");
					block.set("prop:displayMode", op.after);
				} else if (op.kind === "set_metadata") {
					var keys = Object.keys(op.after || {});
					if (keys.length !== 1) throw new Error("metadata operation requires one key");
					block.set(keys[0], op.after[keys[0]]);
				} else {
					throw new Error("unsupported transform operation: " + op.kind);
				}
				applied.push(op.id);
			}
			return JSON.stringify({applied_ids: applied});
		})()
	`, doc, rawOps, cardCreateScriptHelpers)
}
