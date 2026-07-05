package canvaswrite

import "testing"

func TestDiffSearchResultsReportsCoreCategories(t *testing.T) {
	before := SearchResult{
		SourceMode: "snapshot",
		Count:      4,
		Entities: []SearchEntity{
			{ID: "card", Kind: "card", Text: "Old", DisplayMode: "edgeless", XYWH: []float64{0, 0, 100, 100}, SourceID: "img-old"},
			{ID: "edge", Kind: "connector", ConnectorSource: "a", ConnectorTarget: "b", ConnectorLabel: "Old Label", Text: "Old Label"},
			{ID: "broken", Kind: "block", IntegrityIssues: []string{"missing_children"}},
			{ID: "removed", Kind: "block"},
		},
	}
	after := SearchResult{
		SourceMode: "snapshot",
		Count:      4,
		Entities: []SearchEntity{
			{ID: "added", Kind: "block"},
			{ID: "card", Kind: "card", Text: "New", DisplayMode: "embed", XYWH: []float64{10, 0, 100, 100}, SourceID: "img-new"},
			{ID: "edge", Kind: "connector", ConnectorSource: "a", ConnectorTarget: "c", ConnectorLabel: "New Label", Text: "New Label"},
			{ID: "broken", Kind: "block", IntegrityIssues: []string{"unreachable_block"}},
		},
	}

	result := DiffSearchResults("doc-1", before, after)
	for _, category := range []string{
		"added",
		"removed",
		"text_changed",
		"geometry_changed",
		"display_mode_changed",
		"relinked",
		"connector_label_changed",
		"image_source_changed",
		"integrity_changed",
		"unreachable",
	} {
		if result.Summary[category] == 0 {
			t.Fatalf("missing category %q in %#v", category, result.Summary)
		}
	}
	if result.OK {
		t.Fatal("OK = true, want false")
	}
}

func TestDiffSearchOptionsRequiresBeforeSource(t *testing.T) {
	_, _, err := diffSearchOptions(DiffOptions{WorkspaceID: "ws", DocID: "doc"})
	if err == nil {
		t.Fatal("diffSearchOptions error = nil, want before-source error")
	}
}

func TestDiffSearchOptionsSupportsSnapshotVsSnapshotWithoutWorkspace(t *testing.T) {
	before, after, err := diffSearchOptions(DiffOptions{BeforeSnapshotFile: "old.bin", AfterSnapshotFile: "new.bin"})
	if err != nil {
		t.Fatalf("diffSearchOptions error: %v", err)
	}
	if before.SnapshotFile != "old.bin" || after.SnapshotFile != "new.bin" {
		t.Fatalf("before/after = %#v %#v, want snapshot pair", before, after)
	}
}
