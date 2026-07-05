package canvaswrite

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
	"unicode/utf8"

	"affine-pp-cli/internal/config"
	"affine-pp-cli/internal/yjs"
)

type ConnectorLabelOptions struct {
	DocID        string
	IDs          []string
	Labels       []string
	Text         string
	AllowEmpty   bool
	BackupTarget string
}

type ConnectorLabelPlan struct {
	PlanType     string                    `json:"plan_type"`
	PlanID       string                    `json:"plan_id"`
	DryRun       bool                      `json:"dry_run"`
	DocID        string                    `json:"doc_id,omitempty"`
	Source       DiffSource                `json:"source"`
	AffectedIDs  []string                  `json:"affected_ids"`
	Operations   []ConnectorLabelOperation `json:"operations"`
	Integrity    DocIntegrityResult        `json:"integrity"`
	BackupTarget string                    `json:"backup_target,omitempty"`
	Rollback     TransformProof            `json:"rollback"`
	Proof        TransformProof            `json:"proof"`
	Warnings     []string                  `json:"warnings,omitempty"`
}

type ConnectorLabelOperation struct {
	ID         string         `json:"id"`
	Source     string         `json:"source,omitempty"`
	Target     string         `json:"target,omitempty"`
	Before     string         `json:"before"`
	After      string         `json:"after"`
	LabelStyle map[string]any `json:"label_style,omitempty"`
}

type ConnectorLabelApplyOptions struct {
	WorkspaceID string
	DocID       string
	BackupDir   string
}

type ConnectorLabelApplyResult struct {
	PlanType            string                    `json:"plan_type"`
	PlanID              string                    `json:"plan_id"`
	DocID               string                    `json:"doc_id"`
	DryRun              bool                      `json:"dry_run"`
	Applied             bool                      `json:"applied"`
	BackupDir           string                    `json:"backup_dir"`
	AffectedIDs         []string                  `json:"affected_ids"`
	Operations          []ConnectorLabelOperation `json:"operations"`
	SemanticDiffPreview []DiffIssue               `json:"semantic_diff_preview"`
	Before              DocIntegrityResult        `json:"before"`
	After               DocIntegrityResult        `json:"after"`
	Proof               TransformProof            `json:"proof"`
}

func BuildConnectorLabelPlan(selectors SearchResult, opts ConnectorLabelOptions) (ConnectorLabelPlan, error) {
	labels, err := parseConnectorLabelUpdates(opts)
	if err != nil {
		return ConnectorLabelPlan{}, err
	}
	entities := selectedConnectorEntities(selectors.Entities, opts.IDs)
	if len(entities) == 0 {
		return ConnectorLabelPlan{}, fmt.Errorf("no selected connectors; pass connector selectors or --id")
	}
	var ops []ConnectorLabelOperation
	for _, entity := range entities {
		if entity.Kind != "connector" {
			return ConnectorLabelPlan{}, fmt.Errorf("selector %q is %q, want connector", entity.ID, entity.Kind)
		}
		after, ok := labels[entity.ID]
		if !ok {
			return ConnectorLabelPlan{}, fmt.Errorf("missing label for connector %q", entity.ID)
		}
		if err := validateConnectorLabelText(after, opts.AllowEmpty); err != nil {
			return ConnectorLabelPlan{}, fmt.Errorf("connector %q: %w", entity.ID, err)
		}
		ops = append(ops, ConnectorLabelOperation{
			ID:         entity.ID,
			Source:     entity.ConnectorSource,
			Target:     entity.ConnectorTarget,
			Before:     entity.ConnectorLabel,
			After:      after,
			LabelStyle: entity.LabelStyle,
		})
	}
	if len(ops) == 0 {
		return ConnectorLabelPlan{}, fmt.Errorf("no connector label operation requested")
	}
	affected := connectorLabelAffectedIDs(ops)
	plan := ConnectorLabelPlan{
		PlanType:     "canvas_connector_labels",
		DryRun:       true,
		DocID:        opts.DocID,
		Source:       DiffSource{Mode: selectors.SourceMode, Timestamp: selectors.Timestamp, Count: selectors.Count},
		AffectedIDs:  affected,
		Operations:   ops,
		Integrity:    DocIntegrityResult{DocID: opts.DocID, OK: true, Summary: map[string]int{}},
		BackupTarget: opts.BackupTarget,
		Rollback:     TransformProof{Required: true, Fields: []string{"before_snapshot", "delta", "affected_ids", "previous_labels"}},
		Proof:        TransformProof{Required: true, Fields: []string{"pre_integrity", "post_integrity", "reload_label_verification"}},
	}
	plan.PlanID = connectorLabelPlanID(plan)
	return plan, nil
}

func parseConnectorLabelUpdates(opts ConnectorLabelOptions) (map[string]string, error) {
	out := map[string]string{}
	for _, raw := range opts.Labels {
		id, label, ok := strings.Cut(raw, "=")
		id = strings.TrimSpace(id)
		if !ok || id == "" {
			return nil, fmt.Errorf("--label must use connector-id=text")
		}
		out[id] = strings.TrimSpace(label)
	}
	if opts.Text != "" || opts.AllowEmpty {
		if len(opts.IDs) == 0 {
			return nil, fmt.Errorf("--text requires at least one --id")
		}
		for _, id := range opts.IDs {
			id = strings.TrimSpace(id)
			if id != "" {
				out[id] = strings.TrimSpace(opts.Text)
			}
		}
	}
	return out, nil
}

func validateConnectorLabelText(label string, allowEmpty bool) error {
	if !utf8.ValidString(label) {
		return fmt.Errorf("label must be valid UTF-8")
	}
	if strings.ContainsAny(label, "\r\n") {
		return fmt.Errorf("multi-line labels are not supported")
	}
	if strings.TrimSpace(label) == "" && !allowEmpty {
		return fmt.Errorf("empty label requires --allow-empty")
	}
	return nil
}

func selectedConnectorEntities(entities []SearchEntity, ids []string) []SearchEntity {
	if len(ids) == 0 {
		var out []SearchEntity
		for _, entity := range entities {
			if entity.Kind == "connector" {
				out = append(out, entity)
			}
		}
		return out
	}
	byID := searchEntitiesByID(entities)
	var out []SearchEntity
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if entity, ok := byID[id]; ok {
			out = append(out, entity)
		} else {
			out = append(out, SearchEntity{ID: id, Kind: "connector"})
		}
	}
	return out
}

func connectorLabelAffectedIDs(ops []ConnectorLabelOperation) []string {
	seen := map[string]bool{}
	for _, op := range ops {
		seen[op.ID] = true
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func connectorLabelPlanID(plan ConnectorLabelPlan) string {
	h := fnv.New32a()
	for _, id := range plan.AffectedIDs {
		_, _ = h.Write([]byte(id))
	}
	for _, op := range plan.Operations {
		_, _ = h.Write([]byte(op.ID + op.Before + ">" + op.After))
	}
	return fmt.Sprintf("canvas-labels-%08x", h.Sum32())
}

func ValidateConnectorLabelApply(plan ConnectorLabelPlan, opts ConnectorLabelApplyOptions) error {
	if plan.PlanType != "canvas_connector_labels" {
		return fmt.Errorf("live canvas connector label apply requires plan_type canvas_connector_labels")
	}
	if strings.TrimSpace(plan.PlanID) == "" {
		return fmt.Errorf("canvas connector label plan requires plan_id")
	}
	if len(plan.Operations) == 0 {
		return fmt.Errorf("canvas connector label plan requires operations")
	}
	if !plan.Integrity.OK {
		return fmt.Errorf("canvas connector label plan integrity is not OK")
	}
	for _, op := range plan.Operations {
		if strings.TrimSpace(op.ID) == "" {
			return fmt.Errorf("connector label operation requires id")
		}
		if err := validateConnectorLabelText(op.After, true); err != nil {
			return fmt.Errorf("connector %q: %w", op.ID, err)
		}
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

func ApplyConnectorLabelPlan(cfg *config.Config, plan ConnectorLabelPlan, opts ConnectorLabelApplyOptions) (ConnectorLabelApplyResult, error) {
	if err := ValidateConnectorLabelApply(plan, opts); err != nil {
		return ConnectorLabelApplyResult{}, err
	}
	client, err := connect(cfg, opts.WorkspaceID)
	if err != nil {
		return ConnectorLabelApplyResult{}, err
	}
	defer client.Close()

	engine, err := yjs.NewEngine()
	if err != nil {
		return ConnectorLabelApplyResult{}, err
	}
	loaded, err := client.LoadDoc(opts.WorkspaceID, opts.DocID)
	if err != nil {
		return ConnectorLabelApplyResult{}, err
	}
	if loaded.Missing == "" {
		return ConnectorLabelApplyResult{}, fmt.Errorf("document returned empty snapshot")
	}
	doc, err := engine.ApplyBase64Update(loaded.Missing)
	if err != nil {
		return ConnectorLabelApplyResult{}, err
	}
	blocks, err := engine.ReadBlocks(doc)
	if err != nil {
		return ConnectorLabelApplyResult{}, err
	}
	before := CheckBlocksIntegrity(opts.DocID, blocks)
	if !before.OK {
		return ConnectorLabelApplyResult{}, fmt.Errorf("canvas doc integrity failed before connector label apply: %s", integritySummary(before))
	}
	if err := writeRepairBackup(opts.BackupDir, "before", loaded.Missing); err != nil {
		return ConnectorLabelApplyResult{}, err
	}
	stateVector, err := engine.SaveStateVector(doc)
	if err != nil {
		return ConnectorLabelApplyResult{}, err
	}
	if _, err := engine.RunScript(connectorLabelApplyScript(doc, plan.Operations)); err != nil {
		return ConnectorLabelApplyResult{}, fmt.Errorf("apply connector label operations: %w", err)
	}
	blocks, err = engine.ReadBlocks(doc)
	if err != nil {
		return ConnectorLabelApplyResult{}, err
	}
	if err := EnsureBlocksIntegrity(opts.DocID, blocks, "after local connector label apply"); err != nil {
		return ConnectorLabelApplyResult{}, err
	}
	delta, err := engine.EncodeDelta(doc, stateVector)
	if err != nil {
		return ConnectorLabelApplyResult{}, err
	}
	if err := writeRepairBackup(opts.BackupDir, "delta", delta); err != nil {
		return ConnectorLabelApplyResult{}, err
	}
	if err := client.PushDocUpdate(opts.WorkspaceID, opts.DocID, delta); err != nil {
		return ConnectorLabelApplyResult{}, err
	}
	reloaded, err := client.LoadDoc(opts.WorkspaceID, opts.DocID)
	if err != nil {
		return ConnectorLabelApplyResult{}, err
	}
	reloadedDoc, err := engine.ApplyBase64Update(reloaded.Missing)
	if err != nil {
		return ConnectorLabelApplyResult{}, err
	}
	reloadedBlocks, err := engine.ReadBlocks(reloadedDoc)
	if err != nil {
		return ConnectorLabelApplyResult{}, err
	}
	after := CheckBlocksIntegrity(opts.DocID, reloadedBlocks)
	if !after.OK {
		return ConnectorLabelApplyResult{}, fmt.Errorf("canvas doc integrity failed after pushed connector labels: %s", integritySummary(after))
	}
	if err := verifyConnectorLabels(opts.DocID, reloadedBlocks, plan.Operations); err != nil {
		return ConnectorLabelApplyResult{}, err
	}
	return ConnectorLabelApplyResult{
		PlanType:            plan.PlanType,
		PlanID:              plan.PlanID,
		DocID:               opts.DocID,
		DryRun:              false,
		Applied:             true,
		BackupDir:           opts.BackupDir,
		AffectedIDs:         plan.AffectedIDs,
		Operations:          plan.Operations,
		SemanticDiffPreview: ConnectorLabelDiffPreview(plan.Operations),
		Before:              before,
		After:               after,
		Proof:               TransformProof{Required: true, Fields: []string{"before.bin", "before.b64", "delta.bin", "delta.b64", "post_integrity", "reload_label_verification"}},
	}, nil
}

func verifyConnectorLabels(docID string, blocks map[string]map[string]any, ops []ConnectorLabelOperation) error {
	result, err := SearchBlocks(docID, blocks, SearchOptions{Flavour: "affine:connector"})
	if err != nil {
		return err
	}
	byID := searchEntitiesByID(result.Entities)
	for _, op := range ops {
		entity, ok := byID[op.ID]
		if !ok {
			return fmt.Errorf("connector %q missing after reload", op.ID)
		}
		if entity.ConnectorLabel != op.After {
			return fmt.Errorf("connector %q label after reload = %q, want %q", op.ID, entity.ConnectorLabel, op.After)
		}
	}
	return nil
}

func ConnectorLabelDiffPreview(ops []ConnectorLabelOperation) []DiffIssue {
	issues := make([]DiffIssue, 0, len(ops))
	for _, op := range ops {
		issues = append(issues, DiffIssue{
			Category:            "connector_label_changed",
			Severity:            "info",
			ID:                  op.ID,
			Before:              op.Before,
			After:               op.After,
			SuggestedNextAction: "Verify this planned connector label before live apply.",
		})
	}
	return issues
}

func connectorLabelApplyScript(doc int, ops []ConnectorLabelOperation) string {
	rawOps, _ := json.Marshal(ops)
	return fmt.Sprintf(`
		(function() {
			var doc = globalThis._docs[%d];
			var blocks = doc.getMap("blocks");
			var ops = %s;
			function mapToPlain(v) {
				if (v instanceof Y.Map) {
					var obj = {};
					v.forEach(function(val, key) { obj[key] = mapToPlain(val); });
					return obj;
				}
				return v;
			}
			function nestedId(raw) {
				if (!raw) return "";
				if (raw instanceof Y.Map) return String(raw.get("id") || "");
				if (typeof raw === "object") return String(raw.id || "");
				return "";
			}
			function labelText(element) {
				var value = element.get("text");
				if (value === undefined || value === null) value = element.get("label");
				if (value === undefined || value === null) value = element.get("labelText");
				return value === undefined || value === null ? "" : String(value);
			}
			function surfaceElements() {
				var surfaces = [];
				blocks.forEach(function(block) {
					if (block instanceof Y.Map && block.get("sys:flavour") === "affine:surface") surfaces.push(block);
				});
				return surfaces;
			}
			function elementsMap(surface) {
				var raw = surface.get("prop:elements");
				var boxed = raw instanceof Y.Map && raw.get("type") === "$blocksuite:internal:native$";
				var value = boxed ? raw.get("value") : raw;
				if (value instanceof Y.Map) return value;
				throw new Error("surface elements map not found");
			}
			function findConnector(id) {
				var surfaces = surfaceElements();
				for (var i = 0; i < surfaces.length; i++) {
					var value = elementsMap(surfaces[i]);
					var element = value.get(id);
					if (element instanceof Y.Map) return element;
				}
				throw new Error("connector not found: " + id);
			}
			var applied = [];
			for (var i = 0; i < ops.length; i++) {
				var op = ops[i];
				var element = findConnector(op.id);
				if (element.get("type") !== "connector") throw new Error("element is not connector: " + op.id);
				var source = nestedId(element.get("source"));
				var target = nestedId(element.get("target"));
				if (op.source && source !== op.source) throw new Error("connector source changed for " + op.id);
				if (op.target && target !== op.target) throw new Error("connector target changed for " + op.id);
				if (labelText(element) !== String(op.before || "")) throw new Error("connector label changed before apply: " + op.id);
				element.set("text", String(op.after || ""));
				applied.push({id: op.id, source: source, target: target, label: String(op.after || ""), labelStyle: mapToPlain(element.get("labelStyle"))});
			}
			return JSON.stringify({applied_labels: applied});
		})()
	`, doc, string(rawOps))
}
