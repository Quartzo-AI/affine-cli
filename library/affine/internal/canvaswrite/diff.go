package canvaswrite

import (
	"fmt"
	"reflect"
	"sort"

	"affine-pp-cli/internal/config"
)

type DiffOptions struct {
	WorkspaceID        string
	DocID              string
	BeforeSnapshotFile string
	AfterSnapshotFile  string
	BeforeTimestamp    string
	AfterTimestamp     string
	TextLimit          int
}

type DiffResult struct {
	DocID      string         `json:"doc_id,omitempty"`
	Before     DiffSource     `json:"before"`
	After      DiffSource     `json:"after"`
	OK         bool           `json:"ok"`
	IssueCount int            `json:"issue_count"`
	Issues     []DiffIssue    `json:"issues,omitempty"`
	Summary    map[string]int `json:"summary"`
}

type DiffSource struct {
	Mode      string `json:"mode"`
	Timestamp string `json:"timestamp,omitempty"`
	Count     int    `json:"count"`
}

type DiffIssue struct {
	Category            string `json:"category"`
	Severity            string `json:"severity"`
	ID                  string `json:"id"`
	Before              any    `json:"before,omitempty"`
	After               any    `json:"after,omitempty"`
	SuggestedNextAction string `json:"suggested_next_action"`
}

func DiffDoc(cfg *config.Config, opts DiffOptions) (DiffResult, error) {
	beforeOpts, afterOpts, err := diffSearchOptions(opts)
	if err != nil {
		return DiffResult{}, err
	}
	before, err := SearchDoc(cfg, beforeOpts)
	if err != nil {
		return DiffResult{}, fmt.Errorf("load before: %w", err)
	}
	after, err := SearchDoc(cfg, afterOpts)
	if err != nil {
		return DiffResult{}, fmt.Errorf("load after: %w", err)
	}
	return DiffSearchResults(opts.DocID, before, after), nil
}

func DiffSearchResults(docID string, before, after SearchResult) DiffResult {
	result := DiffResult{
		DocID:   docID,
		Before:  DiffSource{Mode: before.SourceMode, Timestamp: before.Timestamp, Count: before.Count},
		After:   DiffSource{Mode: after.SourceMode, Timestamp: after.Timestamp, Count: after.Count},
		Summary: map[string]int{},
	}
	beforeByID := searchEntitiesByID(before.Entities)
	afterByID := searchEntitiesByID(after.Entities)
	ids := unionEntityIDs(beforeByID, afterByID)
	for _, id := range ids {
		b, bOK := beforeByID[id]
		a, aOK := afterByID[id]
		switch {
		case !bOK:
			result.addDiff(DiffIssue{Category: "added", Severity: "info", ID: id, After: a, SuggestedNextAction: "Review whether the new entity is expected."})
			continue
		case !aOK:
			result.addDiff(DiffIssue{Category: "removed", Severity: "warning", ID: id, Before: b, SuggestedNextAction: "Verify the entity was intentionally removed."})
			continue
		}
		compareEntity(&result, b, a)
	}
	result.IssueCount = len(result.Issues)
	result.OK = result.IssueCount == 0
	return result
}

func diffSearchOptions(opts DiffOptions) (SearchOptions, SearchOptions, error) {
	if opts.BeforeSnapshotFile == "" && opts.BeforeTimestamp == "" {
		return SearchOptions{}, SearchOptions{}, fmt.Errorf("set --before-snapshot or --before-timestamp")
	}
	if opts.AfterSnapshotFile != "" && opts.AfterTimestamp != "" {
		return SearchOptions{}, SearchOptions{}, fmt.Errorf("set only one of --after-snapshot or --after-timestamp")
	}
	before := SearchOptions{
		WorkspaceID:  opts.WorkspaceID,
		DocID:        opts.DocID,
		SnapshotFile: opts.BeforeSnapshotFile,
		Timestamp:    opts.BeforeTimestamp,
		TextLimit:    opts.TextLimit,
	}
	after := SearchOptions{
		WorkspaceID:  opts.WorkspaceID,
		DocID:        opts.DocID,
		SnapshotFile: opts.AfterSnapshotFile,
		Timestamp:    opts.AfterTimestamp,
		TextLimit:    opts.TextLimit,
	}
	if before.SnapshotFile == "" && (opts.WorkspaceID == "" || opts.DocID == "") {
		return SearchOptions{}, SearchOptions{}, fmt.Errorf("--workspace and --doc are required for before history/live sources")
	}
	if after.SnapshotFile == "" && (opts.WorkspaceID == "" || opts.DocID == "") {
		return SearchOptions{}, SearchOptions{}, fmt.Errorf("--workspace and --doc are required for after history/live sources")
	}
	return before, after, nil
}

func compareEntity(result *DiffResult, before, after SearchEntity) {
	if before.Kind == "connector" || after.Kind == "connector" {
		if before.ConnectorLabel != after.ConnectorLabel {
			result.addDiff(DiffIssue{Category: "connector_label_changed", Severity: "info", ID: before.ID, Before: before.ConnectorLabel, After: after.ConnectorLabel, SuggestedNextAction: "Review connector label text before applying label updates."})
		}
	} else if before.Text != after.Text {
		result.addDiff(DiffIssue{Category: "text_changed", Severity: "warning", ID: before.ID, Before: before.Text, After: after.Text, SuggestedNextAction: "Review text delta before planning a transform."})
	}
	if !reflect.DeepEqual(before.XYWH, after.XYWH) {
		result.addDiff(DiffIssue{Category: "geometry_changed", Severity: "info", ID: before.ID, Before: before.XYWH, After: after.XYWH, SuggestedNextAction: "Use transform planning before applying geometry changes."})
	}
	if before.DisplayMode != after.DisplayMode {
		result.addDiff(DiffIssue{Category: "display_mode_changed", Severity: "info", ID: before.ID, Before: before.DisplayMode, After: after.DisplayMode, SuggestedNextAction: "Verify the card display mode is expected."})
	}
	if before.ConnectorSource != after.ConnectorSource || before.ConnectorTarget != after.ConnectorTarget {
		result.addDiff(DiffIssue{Category: "relinked", Severity: "warning", ID: before.ID, Before: []string{before.ConnectorSource, before.ConnectorTarget}, After: []string{after.ConnectorSource, after.ConnectorTarget}, SuggestedNextAction: "Inspect connector endpoints before planning relink operations."})
	}
	if before.SourceID != after.SourceID {
		result.addDiff(DiffIssue{Category: "image_source_changed", Severity: "info", ID: before.ID, Before: before.SourceID, After: after.SourceID, SuggestedNextAction: "Inspect card media before replacing assets."})
	}
	if !reflect.DeepEqual(before.IntegrityIssues, after.IntegrityIssues) {
		result.addDiff(DiffIssue{Category: "integrity_changed", Severity: "error", ID: before.ID, Before: before.IntegrityIssues, After: after.IntegrityIssues, SuggestedNextAction: "Run canvas doc integrity before planning mutations."})
	}
	if hasIssue(after.IntegrityIssues, "unreachable_block") {
		result.addDiff(DiffIssue{Category: "unreachable", Severity: "error", ID: before.ID, After: after.IntegrityIssues, SuggestedNextAction: "Repair document reachability before transform planning."})
	}
	if after.Kind == "connector" && (after.ConnectorSource == "" || after.ConnectorTarget == "") {
		result.addDiff(DiffIssue{Category: "orphaned", Severity: "error", ID: before.ID, After: []string{after.ConnectorSource, after.ConnectorTarget}, SuggestedNextAction: "Fix connector endpoints before transform planning."})
	}
}

func (r *DiffResult) addDiff(issue DiffIssue) {
	r.Issues = append(r.Issues, issue)
	r.Summary[issue.Category]++
}

func searchEntitiesByID(entities []SearchEntity) map[string]SearchEntity {
	out := map[string]SearchEntity{}
	for _, entity := range entities {
		out[entity.ID] = entity
	}
	return out
}

func unionEntityIDs(a, b map[string]SearchEntity) []string {
	seen := map[string]bool{}
	for id := range a {
		seen[id] = true
	}
	for id := range b {
		seen[id] = true
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func hasIssue(issues []string, target string) bool {
	for _, issue := range issues {
		if issue == target {
			return true
		}
	}
	return false
}
