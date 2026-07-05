package canvaswrite

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"affine-pp-cli/internal/config"
	"affine-pp-cli/internal/yjs"
)

type SearchOptions struct {
	WorkspaceID   string
	DocID         string
	SnapshotFile  string
	Timestamp     string
	Text          string
	ID            string
	Flavour       string
	Type          string
	DisplayMode   string
	SourceMode    string
	Bounds        string
	ConnectorFrom string
	ConnectorTo   string
	Limit         int
	TextLimit     int
}

type SearchResult struct {
	DocID      string         `json:"doc_id,omitempty"`
	Timestamp  string         `json:"timestamp,omitempty"`
	SourceMode string         `json:"source_mode"`
	Count      int            `json:"count"`
	Entities   []SearchEntity `json:"entities"`
}

type SearchEntity struct {
	ID              string         `json:"id"`
	Kind            string         `json:"kind"`
	Flavour         string         `json:"flavour,omitempty"`
	Type            string         `json:"type,omitempty"`
	Text            string         `json:"text,omitempty"`
	ConnectorLabel  string         `json:"connector_label,omitempty"`
	LabelStyle      map[string]any `json:"label_style,omitempty"`
	ParentID        string         `json:"parent_id,omitempty"`
	ChildIDs        []string       `json:"child_ids,omitempty"`
	DisplayMode     string         `json:"display_mode,omitempty"`
	Hidden          bool           `json:"hidden,omitempty"`
	XYWH            []float64      `json:"xywh,omitempty"`
	SourceID        string         `json:"source_id,omitempty"`
	ConnectorSource string         `json:"connector_source,omitempty"`
	ConnectorTarget string         `json:"connector_target,omitempty"`
	IntegrityIssues []string       `json:"integrity_issues,omitempty"`
}

func SearchDoc(cfg *config.Config, opts SearchOptions) (SearchResult, error) {
	if opts.TextLimit <= 0 {
		opts.TextLimit = 240
	}
	if opts.Limit < 0 {
		return SearchResult{}, fmt.Errorf("--limit must be >= 0")
	}
	engine, err := yjs.NewEngine()
	if err != nil {
		return SearchResult{}, err
	}
	blocks, err := auditBlocks(cfg, engine, DocAuditOptions{
		WorkspaceID:  opts.WorkspaceID,
		DocID:        opts.DocID,
		SnapshotFile: opts.SnapshotFile,
		Timestamp:    opts.Timestamp,
	})
	if err != nil {
		return SearchResult{}, err
	}
	return SearchBlocks(opts.DocID, blocks, opts)
}

func SearchBlocks(docID string, blocks map[string]map[string]any, opts SearchOptions) (SearchResult, error) {
	if opts.TextLimit <= 0 {
		opts.TextLimit = 240
	}
	if opts.Limit < 0 {
		return SearchResult{}, fmt.Errorf("--limit must be >= 0")
	}
	sourceMode := canvasSourceMode(opts)
	if opts.SourceMode != "" && opts.SourceMode != sourceMode {
		return SearchResult{DocID: docID, Timestamp: opts.Timestamp, SourceMode: sourceMode}, nil
	}
	bounds, hasBounds, err := parseBounds(opts.Bounds)
	if err != nil {
		return SearchResult{}, err
	}
	issuesByID := searchIntegrityIssues(docID, blocks)
	parentByChild := searchParentByChild(blocks)
	result := SearchResult{DocID: docID, Timestamp: opts.Timestamp, SourceMode: sourceMode}
	add := func(entity SearchEntity) bool {
		if !matchesSearch(entity, opts, bounds, hasBounds) {
			return true
		}
		result.Entities = append(result.Entities, entity)
		return opts.Limit <= 0 || len(result.Entities) < opts.Limit
	}
	for _, id := range sortedBlockIDs(blocks) {
		entity := buildSearchEntity(id, blocks, parentByChild[id], issuesByID[id], opts.TextLimit)
		if !add(entity) {
			result.Count = len(result.Entities)
			return result, nil
		}
	}
	for _, entity := range surfaceConnectorEntities(blocks, opts.TextLimit) {
		if !add(entity) {
			break
		}
	}
	result.Count = len(result.Entities)
	return result, nil
}

func buildSearchEntity(id string, blocks map[string]map[string]any, parentID string, issues []string, textLimit int) SearchEntity {
	block := blocks[id]
	children := childIDs(block)
	entity := SearchEntity{
		ID:              id,
		Kind:            searchKind(block),
		Flavour:         stringField(block, "sys:flavour"),
		Type:            stringField(block, "sys:type"),
		Text:            truncateRunes(searchText(blocks, id, map[string]bool{}), textLimit),
		ParentID:        parentID,
		ChildIDs:        children,
		DisplayMode:     stringField(block, "prop:displayMode"),
		SourceID:        stringField(block, "prop:sourceId"),
		ConnectorSource: stringField(block, "prop:source"),
		ConnectorTarget: stringField(block, "prop:target"),
		IntegrityIssues: issues,
	}
	if hidden, _ := block["prop:hidden"].(bool); hidden {
		entity.Hidden = true
	}
	if xywh, ok := parseXYWH(stringField(block, "prop:xywh")); ok {
		entity.XYWH = xywh
	}
	return entity
}

func matchesSearch(entity SearchEntity, opts SearchOptions, bounds []float64, hasBounds bool) bool {
	if opts.ID != "" && entity.ID != opts.ID {
		return false
	}
	if opts.Flavour != "" && entity.Flavour != opts.Flavour {
		return false
	}
	if opts.Type != "" && entity.Type != opts.Type {
		return false
	}
	if opts.DisplayMode != "" && entity.DisplayMode != opts.DisplayMode {
		return false
	}
	if opts.ConnectorFrom != "" && entity.ConnectorSource != opts.ConnectorFrom {
		return false
	}
	if opts.ConnectorTo != "" && entity.ConnectorTarget != opts.ConnectorTo {
		return false
	}
	if opts.Text != "" && !strings.Contains(strings.ToLower(entity.Text), strings.ToLower(opts.Text)) {
		return false
	}
	if hasBounds && !rectsIntersect(entity.XYWH, bounds) {
		return false
	}
	return true
}

func canvasSourceMode(opts SearchOptions) string {
	switch {
	case opts.SnapshotFile != "":
		return "snapshot"
	case opts.Timestamp != "":
		return "history"
	default:
		return "live"
	}
}

func searchKind(block map[string]any) string {
	if stringField(block, "prop:source") != "" || stringField(block, "prop:target") != "" {
		return "connector"
	}
	switch stringField(block, "sys:flavour") {
	case "affine:note":
		return "card"
	case "affine:page":
		return "page"
	default:
		return "block"
	}
}

func surfaceConnectorEntities(blocks map[string]map[string]any, textLimit int) []SearchEntity {
	var out []SearchEntity
	for _, surfaceID := range sortedBlockIDs(blocks) {
		surface := blocks[surfaceID]
		if stringField(surface, "sys:flavour") != "affine:surface" {
			continue
		}
		elements := nativeElements(surface["prop:elements"])
		for _, id := range sortedAnyKeys(elements) {
			el := mapAny(elements[id])
			if stringField(el, "type") != "connector" {
				continue
			}
			sourceID := nestedID(el["source"])
			targetID := nestedID(el["target"])
			entity := SearchEntity{
				ID:              id,
				Kind:            "connector",
				Flavour:         "affine:connector",
				Text:            truncateRunes(connectorLabelText(el), optsTextLimit(textLimit)),
				ConnectorLabel:  connectorLabelText(el),
				LabelStyle:      mapAny(el["labelStyle"]),
				ParentID:        surfaceID,
				ConnectorSource: sourceID,
				ConnectorTarget: targetID,
			}
			if xywh, ok := connectorXYWH(blocks[sourceID], blocks[targetID]); ok {
				entity.XYWH = xywh
			}
			out = append(out, entity)
		}
	}
	return out
}

func optsTextLimit(limit int) int {
	if limit <= 0 {
		return 240
	}
	return limit
}

func connectorLabelText(el map[string]any) string {
	for _, key := range []string{"text", "label", "labelText"} {
		if text := stringField(el, key); text != "" {
			return text
		}
	}
	return ""
}

func nativeElements(raw any) map[string]any {
	obj := mapAny(raw)
	if obj == nil {
		return nil
	}
	if obj["type"] == "$blocksuite:internal:native$" {
		return mapAny(obj["value"])
	}
	return obj
}

func mapAny(raw any) map[string]any {
	if raw == nil {
		return nil
	}
	if m, ok := raw.(map[string]any); ok {
		return m
	}
	return nil
}

func nestedID(raw any) string {
	return stringField(mapAny(raw), "id")
}

func connectorXYWH(source, target map[string]any) ([]float64, bool) {
	a, okA := parseXYWH(stringField(source, "prop:xywh"))
	b, okB := parseXYWH(stringField(target, "prop:xywh"))
	if !okA || !okB {
		return nil, false
	}
	ax, ay := a[0]+a[2]/2, a[1]+a[3]/2
	bx, by := b[0]+b[2]/2, b[1]+b[3]/2
	x, y := math.Min(ax, bx), math.Min(ay, by)
	w, h := math.Max(1, math.Abs(bx-ax)), math.Max(1, math.Abs(by-ay))
	return []float64{x, y, w, h}, true
}

func sortedAnyKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func searchText(blocks map[string]map[string]any, id string, seen map[string]bool) string {
	if id == "" || seen[id] {
		return ""
	}
	seen[id] = true
	block := blocks[id]
	if block == nil {
		return ""
	}
	parts := []string{stringField(block, "prop:text")}
	for _, childID := range childIDs(block) {
		if text := searchText(blocks, childID, seen); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func childIDs(block map[string]any) []string {
	children, _ := block["sys:children"].([]any)
	out := make([]string, 0, len(children))
	for _, raw := range children {
		if id, _ := raw.(string); id != "" {
			out = append(out, id)
		}
	}
	return out
}

func searchParentByChild(blocks map[string]map[string]any) map[string]string {
	out := map[string]string{}
	for id, block := range blocks {
		for _, childID := range childIDs(block) {
			if out[childID] == "" {
				out[childID] = id
			}
		}
	}
	return out
}

func searchIntegrityIssues(docID string, blocks map[string]map[string]any) map[string][]string {
	out := map[string][]string{}
	for _, issue := range CheckBlocksIntegrity(docID, blocks).Issues {
		id := issue.ID
		if id == "" {
			id = issue.Child
		}
		if id == "" {
			id = issue.Parent
		}
		if id != "" {
			out[id] = append(out[id], issue.Code)
		}
	}
	return out
}

func parseBounds(s string) ([]float64, bool, error) {
	if strings.TrimSpace(s) == "" {
		return nil, false, nil
	}
	xywh, ok := parseXYWH(s)
	if !ok {
		return nil, false, fmt.Errorf("--bounds must be x,y,w,h")
	}
	return xywh, true, nil
}

func parseXYWH(s string) ([]float64, bool) {
	s = strings.TrimSpace(strings.Trim(s, "[]"))
	if s == "" {
		return nil, false
	}
	parts := strings.Split(s, ",")
	if len(parts) != 4 {
		return nil, false
	}
	out := make([]float64, 4)
	for i, part := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
		if err != nil {
			return nil, false
		}
		out[i] = v
	}
	return out, true
}

func rectsIntersect(a, b []float64) bool {
	if len(a) != 4 || len(b) != 4 {
		return false
	}
	return a[0] < b[0]+b[2] && a[0]+a[2] > b[0] && a[1] < b[1]+b[3] && a[1]+a[3] > b[1]
}
