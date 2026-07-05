package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRootIncludesCanvasCommands(t *testing.T) {
	root := RootCmd()
	for _, args := range [][]string{
		{"canvas"},
		{"canvas", "plan"},
		{"canvas", "search"},
		{"canvas", "diff"},
		{"canvas", "transform"},
		{"canvas", "label-plan"},
		{"canvas", "model"},
		{"canvas", "validate"},
		{"canvas", "apply"},
		{"canvas", "example"},
		{"canvas", "doc", "integrity"},
		{"canvas", "doc", "repair"},
	} {
		cmd, _, err := root.Find(args)
		if err != nil {
			t.Fatalf("Find(%v) error: %v", args, err)
		}
		if cmd == nil || cmd.Name() != args[len(args)-1] {
			t.Fatalf("Find(%v) = %v, want %q", args, cmd, args[len(args)-1])
		}
	}
}

func TestCanvasApplyAcceptsConnectorLabelPlan(t *testing.T) {
	root := RootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetIn(strings.NewReader(`{
		"plan_type": "canvas_connector_labels",
		"plan_id": "canvas-labels-test",
		"dry_run": true,
		"affected_ids": ["edge"],
		"operations": [{"id": "edge", "source": "a", "target": "b", "before": "Old", "after": "New"}]
	}`))
	root.SetArgs([]string{"canvas", "apply", "--dry-run", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute error: %v\n%s", err, out.String())
	}
	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out.String())
	}
	if got["plan_type"] != "canvas_connector_labels" || got["live_write_supported"] != true {
		t.Fatalf("apply output = %#v, want connector label dry-run with gated live support", got)
	}
}

func TestCanvasApplyAcceptsTransformPlan(t *testing.T) {
	root := RootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetIn(strings.NewReader(`{
		"plan_type": "canvas_transform",
		"plan_id": "canvas-transform-test",
		"dry_run": true,
		"affected_ids": ["card"],
		"operations": [{"kind": "set_display_mode", "id": "card", "after": "embed"}]
	}`))
	root.SetArgs([]string{"canvas", "apply", "--dry-run", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute error: %v\n%s", err, out.String())
	}
	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out.String())
	}
	if got["plan_type"] != "canvas_transform" || got["live_write_supported"] != true {
		t.Fatalf("apply output = %#v, want transform dry-run with gated live support", got)
	}
	if _, ok := got["semantic_diff_preview"].([]any); !ok {
		t.Fatalf("apply output = %#v, want semantic_diff_preview", got)
	}
}

func TestCanvasApplyLiveRequiresConfirmationBeforeNetwork(t *testing.T) {
	root := RootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetIn(strings.NewReader(`{
		"plan_type": "canvas_transform",
		"plan_id": "canvas-transform-test",
		"dry_run": true,
		"doc_id": "doc",
		"integrity": {"ok": true},
		"affected_ids": ["card"],
		"operations": [{"kind": "set_display_mode", "id": "card", "after": "embed"}]
	}`))
	root.SetArgs([]string{"canvas", "apply", "--live", "--workspace", "workspace", "--doc", "doc", "--backup-dir", t.TempDir(), "--json"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "confirmation required") {
		t.Fatalf("Execute error = %v, want confirmation required before live apply\n%s", err, out.String())
	}
}

func TestCanvasApplyLiveRequiresBackupDirBeforeNetwork(t *testing.T) {
	root := RootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetIn(strings.NewReader(`{
		"plan_type": "canvas_transform",
		"plan_id": "canvas-transform-test",
		"dry_run": true,
		"doc_id": "doc",
		"integrity": {"ok": true},
		"affected_ids": ["card"],
		"operations": [{"kind": "set_display_mode", "id": "card", "after": "embed"}]
	}`))
	root.SetArgs([]string{"canvas", "apply", "--live", "--workspace", "workspace", "--doc", "doc", "--yes", "--json"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "--backup-dir is required") {
		t.Fatalf("Execute error = %v, want backup-dir gate before live apply\n%s", err, out.String())
	}
}

func TestCanvasApplyLayoutLiveRequiresBackupDirBeforeNetwork(t *testing.T) {
	root := RootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetIn(strings.NewReader(`{
		"frame": "LIVE ZAP",
		"orientation": "vertical",
		"nodes": [{"id": "live-zap", "text": "Live Zap", "w": 360, "h": 220}],
		"connections": []
	}`))
	root.SetArgs([]string{"canvas", "apply", "--live", "--workspace", "workspace", "--doc", "doc", "--yes", "--json"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "--backup-dir is required") {
		t.Fatalf("Execute error = %v, want backup-dir gate before live layout apply\n%s", err, out.String())
	}
}

func TestBuildCanvasPlanVerticalDefault(t *testing.T) {
	plan := buildCanvasPlan(canvasBuildSpec{
		Frame: "TRABALHO",
		Hub:   "Deploy",
		Levels: [][]string{
			{"Deploy"},
			{"Hetzner", "Contabo"},
		},
	})

	if plan.Orientation != "vertical" {
		t.Fatalf("orientation = %q, want vertical", plan.Orientation)
	}
	if len(plan.Nodes) != 3 {
		t.Fatalf("nodes = %d, want 3", len(plan.Nodes))
	}
	if plan.Nodes[0].Role != "hub" {
		t.Fatalf("hub role = %q", plan.Nodes[0].Role)
	}
	if !(plan.Nodes[1].Y > plan.Nodes[0].Y) {
		t.Fatalf("vertical child Y = %v, hub Y = %v", plan.Nodes[1].Y, plan.Nodes[0].Y)
	}
	if len(plan.Connections) != 2 {
		t.Fatalf("connections = %d, want 2", len(plan.Connections))
	}
}

func TestBuildCanvasPlanHorizontal(t *testing.T) {
	plan := buildCanvasPlan(canvasBuildSpec{
		Orientation: "horizontal",
		Hub:         "Coolify",
		Levels: [][]string{
			{"Coolify"},
			{"App A", "App B"},
		},
	})

	if plan.Orientation != "horizontal" {
		t.Fatalf("orientation = %q, want horizontal", plan.Orientation)
	}
	if !(plan.Nodes[1].X > plan.Nodes[0].X) {
		t.Fatalf("horizontal child X = %v, hub X = %v", plan.Nodes[1].X, plan.Nodes[0].X)
	}
}

func TestReadCanvasModelNestedPropsConnection(t *testing.T) {
	input := `{
		"blocks": [
			{"id": "a", "text": "Alpha", "props": {"xywh": "[0,0,360,220]"}},
			{"id": "b", "title": "Beta", "prop:xywh": "[0,360,360,220]"}
		],
		"edges": [
			{"id": "e1", "type": "connector", "props": {"source": "a", "target": "b", "xywh": "[0,0,1,1]"}}
		]
	}`
	model, err := readCanvasModel("", strings.NewReader(input))
	if err != nil {
		t.Fatalf("readCanvasModel error: %v", err)
	}
	if len(model.Nodes) != 2 {
		t.Fatalf("nodes = %d, want 2", len(model.Nodes))
	}
	if len(model.Connections) != 1 {
		t.Fatalf("connections = %d, want 1", len(model.Connections))
	}
	if model.Connections[0]["source"] != "a" || model.Connections[0]["target"] != "b" {
		t.Fatalf("connection = %#v, want source a target b", model.Connections[0])
	}
}

func TestValidateCanvasPlanDetectsOrphanConnection(t *testing.T) {
	report := validateCanvasPlan(canvasPlan{
		Frame:       "TRABALHO",
		Orientation: "vertical",
		Nodes: []canvasNode{
			{ID: "hub", Text: "Hub", W: 360, H: 220},
		},
		Connections: []canvasConnection{
			{From: "Hub", To: "Missing"},
		},
	})

	if report.OK {
		t.Fatalf("OK = true, want false")
	}
	if report.IssueCount == 0 {
		t.Fatalf("IssueCount = 0, want at least 1")
	}
	if report.Issues[0].Code != "orphan_connection" {
		t.Fatalf("first issue code = %q, want orphan_connection", report.Issues[0].Code)
	}
}

func TestValidateCanvasPlanAllowsConnectionOnlyApply(t *testing.T) {
	report := validateCanvasPlan(canvasPlan{
		Connections: []canvasConnection{
			{From: "existing-a", To: "existing-b"},
		},
	})

	if !report.OK {
		t.Fatalf("OK = false, want true: %#v", report.Issues)
	}
}

func TestCanvasLayoutApplyStepsNudgesConnectorsAfterNodes(t *testing.T) {
	steps := canvasLayoutApplySteps(canvasPlan{
		Nodes: []canvasNode{
			{ID: "a", W: 360, H: 220},
		},
		Connections: []canvasConnection{
			{From: "a", To: "b"},
		},
	})
	if len(steps) != 2 {
		t.Fatalf("steps = %d, want 2", len(steps))
	}
	if steps[0].name != "nodes" || len(steps[0].plan.Connections) != 0 {
		t.Fatalf("first step = %#v, want nodes-only", steps[0])
	}
	if steps[1].name != "connectors" || len(steps[1].plan.Nodes) != 0 {
		t.Fatalf("second step = %#v, want connectors-only", steps[1])
	}
}

func TestCanvasPlanConnectorsOnlyPreservesCardPositions(t *testing.T) {
	plan, err := canvasPlanConnectorsOnly(canvasPlan{
		Nodes: []canvasNode{
			{ID: "a", X: 10, Y: 20, W: 360, H: 220},
		},
		Connections: []canvasConnection{
			{From: "a", To: "b"},
		},
	})
	if err != nil {
		t.Fatalf("canvasPlanConnectorsOnly error: %v", err)
	}
	if len(plan.Nodes) != 0 {
		t.Fatalf("nodes = %#v, want none", plan.Nodes)
	}
	if len(plan.Connections) != 1 {
		t.Fatalf("connections = %d, want 1", len(plan.Connections))
	}
}

func TestCanvasPlanConnectorsOnlyRequiresConnections(t *testing.T) {
	_, err := canvasPlanConnectorsOnly(canvasPlan{
		Nodes: []canvasNode{{ID: "a"}},
	})
	if err == nil {
		t.Fatal("error = nil, want missing connections error")
	}
}

func TestReadAllOrFileReadsDashFromStdin(t *testing.T) {
	got, err := readAllOrFile("-", strings.NewReader(`{"ok":true}`))
	if err != nil {
		t.Fatalf("readAllOrFile error: %v", err)
	}
	if string(got) != `{"ok":true}` {
		t.Fatalf("got %q, want stdin payload", string(got))
	}
}
