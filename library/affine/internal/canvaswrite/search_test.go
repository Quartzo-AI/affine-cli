package canvaswrite

import "testing"

func TestSearchBlocksFindsCardByDescendantText(t *testing.T) {
	result, err := SearchBlocks("doc-1", map[string]map[string]any{
		"page": {
			"sys:id":       "page",
			"sys:flavour":  "affine:page",
			"sys:children": []any{"card"},
		},
		"card": {
			"sys:id":           "card",
			"sys:flavour":      "affine:note",
			"sys:children":     []any{"text"},
			"prop:displayMode": "edgeless",
			"prop:xywh":        "[10,20,300,160]",
		},
		"text": {
			"sys:id":       "text",
			"sys:flavour":  "affine:paragraph",
			"sys:children": []any{},
			"prop:text":    "Canvas roadmap",
		},
	}, SearchOptions{Text: "roadmap", Flavour: "affine:note", Bounds: "0,0,100,100"})
	if err != nil {
		t.Fatalf("SearchBlocks error: %v", err)
	}
	if result.Count != 1 {
		t.Fatalf("Count = %d, want 1: %#v", result.Count, result.Entities)
	}
	if got := result.Entities[0]; got.ID != "card" || got.Kind != "card" || got.DisplayMode != "edgeless" {
		t.Fatalf("entity = %#v, want card result", got)
	}
}

func TestSearchBlocksFindsConnectorEndpoint(t *testing.T) {
	result, err := SearchBlocks("doc-1", map[string]map[string]any{
		"page": {
			"sys:id":       "page",
			"sys:flavour":  "affine:page",
			"sys:children": []any{"edge"},
		},
		"edge": {
			"sys:id":       "edge",
			"sys:flavour":  "affine:connector",
			"sys:children": []any{},
			"prop:source":  "a",
			"prop:target":  "b",
		},
	}, SearchOptions{ConnectorFrom: "a", ConnectorTo: "b"})
	if err != nil {
		t.Fatalf("SearchBlocks error: %v", err)
	}
	if result.Count != 1 {
		t.Fatalf("Count = %d, want 1", result.Count)
	}
	if result.Entities[0].Kind != "connector" {
		t.Fatalf("Kind = %q, want connector", result.Entities[0].Kind)
	}
}

func TestSearchBlocksFindsSurfaceConnectorEndpoint(t *testing.T) {
	result, err := SearchBlocks("doc-1", map[string]map[string]any{
		"surface": {
			"sys:id":      "surface",
			"sys:flavour": "affine:surface",
			"prop:elements": map[string]any{
				"type": "$blocksuite:internal:native$",
				"value": map[string]any{
					"edge": map[string]any{
						"id":         "edge",
						"type":       "connector",
						"text":       "Approved Send",
						"labelStyle": map[string]any{"fontSize": float64(16), "textAlign": "center"},
						"source":     map[string]any{"id": "a"},
						"target":     map[string]any{"id": "b"},
					},
				},
			},
		},
		"a": {
			"sys:id":      "a",
			"sys:flavour": "affine:note",
			"prop:xywh":   "[0,0,100,100]",
		},
		"b": {
			"sys:id":      "b",
			"sys:flavour": "affine:note",
			"prop:xywh":   "[200,0,100,100]",
		},
	}, SearchOptions{Flavour: "affine:connector", ConnectorFrom: "a", ConnectorTo: "b", Bounds: "100,0,100,100"})
	if err != nil {
		t.Fatalf("SearchBlocks error: %v", err)
	}
	if result.Count != 1 {
		t.Fatalf("Count = %d, want 1: %#v", result.Count, result.Entities)
	}
	got := result.Entities[0]
	if got.ID != "edge" || got.ParentID != "surface" || got.ConnectorSource != "a" || got.ConnectorTarget != "b" {
		t.Fatalf("entity = %#v, want surface connector", got)
	}
	if got.ConnectorLabel != "Approved Send" || got.Text != "Approved Send" {
		t.Fatalf("connector label = %q text = %q, want Approved Send", got.ConnectorLabel, got.Text)
	}
	if got.LabelStyle["textAlign"] != "center" {
		t.Fatalf("label style = %#v, want textAlign center", got.LabelStyle)
	}
}

func TestSearchBlocksHonorsSourceMode(t *testing.T) {
	result, err := SearchBlocks("doc-1", map[string]map[string]any{
		"page": {
			"sys:id":       "page",
			"sys:flavour":  "affine:page",
			"sys:children": []any{},
		},
	}, SearchOptions{SourceMode: "snapshot"})
	if err != nil {
		t.Fatalf("SearchBlocks error: %v", err)
	}
	if result.SourceMode != "live" || result.Count != 0 {
		t.Fatalf("result = %#v, want live source mismatch with no entities", result)
	}
}
