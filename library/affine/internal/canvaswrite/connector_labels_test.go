package canvaswrite

import (
	"strings"
	"testing"

	"affine-pp-cli/internal/yjs"
)

func TestBuildConnectorLabelPlanFromSelectors(t *testing.T) {
	plan, err := BuildConnectorLabelPlan(SearchResult{
		DocID:      "doc-1",
		SourceMode: "snapshot",
		Count:      1,
		Entities: []SearchEntity{
			{
				ID:              "edge",
				Kind:            "connector",
				ConnectorSource: "a",
				ConnectorTarget: "b",
				ConnectorLabel:  "Old Label",
				LabelStyle:      map[string]any{"fontSize": float64(16)},
			},
		},
	}, ConnectorLabelOptions{
		DocID:  "doc-1",
		Labels: []string{"edge=New Label"},
	})
	if err != nil {
		t.Fatalf("BuildConnectorLabelPlan error: %v", err)
	}
	if plan.PlanType != "canvas_connector_labels" || !plan.DryRun {
		t.Fatalf("plan header = %#v, want connector label dry-run", plan)
	}
	if len(plan.Operations) != 1 {
		t.Fatalf("operations = %d, want 1", len(plan.Operations))
	}
	op := plan.Operations[0]
	if op.ID != "edge" || op.Before != "Old Label" || op.After != "New Label" || op.Source != "a" || op.Target != "b" {
		t.Fatalf("operation = %#v, want reviewed label op", op)
	}
}

func TestBuildConnectorLabelPlanRejectsMultilineLabel(t *testing.T) {
	_, err := BuildConnectorLabelPlan(SearchResult{
		SourceMode: "snapshot",
		Entities:   []SearchEntity{{ID: "edge", Kind: "connector"}},
	}, ConnectorLabelOptions{Labels: []string{"edge=Line 1\nLine 2"}})
	if err == nil || !strings.Contains(err.Error(), "multi-line") {
		t.Fatalf("error = %v, want multi-line rejection", err)
	}
}

func TestValidateConnectorLabelApplyRequiresBackupDir(t *testing.T) {
	err := ValidateConnectorLabelApply(ConnectorLabelPlan{
		PlanType: "canvas_connector_labels",
		PlanID:   "canvas-labels-test",
		Integrity: DocIntegrityResult{
			OK: true,
		},
		Operations: []ConnectorLabelOperation{
			{ID: "edge", Before: "Old", After: "New"},
		},
	}, ConnectorLabelApplyOptions{WorkspaceID: "workspace", DocID: "doc"})
	if err == nil || !strings.Contains(err.Error(), "--backup-dir is required") {
		t.Fatalf("error = %v, want backup-dir gate", err)
	}
}

func TestConnectorLabelApplyScriptChangesOnlyText(t *testing.T) {
	engine, err := yjs.NewEngine()
	if err != nil {
		t.Fatal(err)
	}
	doc, err := engine.NewDoc()
	if err != nil {
		t.Fatal(err)
	}
	_, err = engine.RunScript(`
		(function() {
			var doc = globalThis._docs[0];
			var blocks = doc.getMap("blocks");
			var surface = new Y.Map();
			var boxed = new Y.Map();
			var value = new Y.Map();
			var edge = new Y.Map();
			edge.set("id", "edge");
			edge.set("type", "connector");
			edge.set("text", "Old");
			edge.set("source", {id: "a"});
			edge.set("target", {id: "b"});
			edge.set("stroke", "#929292");
			edge.set("seed", 42);
			edge.set("labelStyle", {fontSize: 16});
			value.set("edge", edge);
			boxed.set("type", "$blocksuite:internal:native$");
			boxed.set("value", value);
			surface.set("sys:id", "surface");
			surface.set("sys:flavour", "affine:surface");
			surface.set("prop:elements", boxed);
			blocks.set("surface", surface);
			return "ok";
		})()
	`)
	if err != nil {
		t.Fatalf("fixture script error: %v", err)
	}
	_, err = engine.RunScript(connectorLabelApplyScript(doc, []ConnectorLabelOperation{
		{ID: "edge", Source: "a", Target: "b", Before: "Old", After: "New"},
	}))
	if err != nil {
		t.Fatalf("connectorLabelApplyScript error: %v", err)
	}
	blocks, err := engine.ReadBlocks(doc)
	if err != nil {
		t.Fatal(err)
	}
	result, err := SearchBlocks("doc", blocks, SearchOptions{Flavour: "affine:connector"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entities) != 1 || result.Entities[0].ConnectorLabel != "New" {
		t.Fatalf("entities = %#v, want updated label", result.Entities)
	}
	elements := blocks["surface"]["prop:elements"].(map[string]any)["value"].(map[string]any)
	edge := elements["edge"].(map[string]any)
	if edge["stroke"] != "#929292" || edge["seed"] != float64(42) {
		t.Fatalf("edge = %#v, want non-label fields preserved", edge)
	}
}
