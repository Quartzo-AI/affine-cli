package canvaswrite

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
	"unicode/utf8"
)

// CardCreateKind is the transform operation kind that creates a new affine:note card.
const CardCreateKind = "create_card"

const (
	cardNoteFlavour      = "affine:note"
	cardParagraphFlavour = "affine:paragraph"
	defaultCardDisplay   = "edgeless"
	defaultCardIndex     = "a0"
)

var cardParagraphTypes = map[string]bool{
	"text":     true,
	"h1":       true,
	"h2":       true,
	"h3":       true,
	"h4":       true,
	"h5":       true,
	"h6":       true,
	"quote":    true,
	"bulleted": true,
	"numbered": true,
	"todo":     true,
	"code":     true,
}

var cardDisplayModes = map[string]bool{
	"edgeless": true,
	"both":     true,
	"page":     true,
}

// CardParagraph is one leaf block inside a created card.
type CardParagraph struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Text string `json:"text"`
}

// CardCreateSpec describes one affine:note card to create on an edgeless doc.
type CardCreateSpec struct {
	Flavour     string          `json:"flavour"`
	X           float64         `json:"x"`
	Y           float64         `json:"y"`
	W           float64         `json:"w"`
	H           float64         `json:"h"`
	DisplayMode string          `json:"display_mode"`
	Index       string          `json:"index,omitempty"`
	Background  string          `json:"background,omitempty"`
	FrameID     string          `json:"frame_id,omitempty"`
	Paragraphs  []CardParagraph `json:"paragraphs"`
}

// CardCreateOptions builds a canvas_transform plan whose operations create cards.
type CardCreateOptions struct {
	DocID        string
	Cards        []CardCreateSpec
	IDs          []string
	BackupTarget string
}

// BuildCardCreatePlan turns card specs into a validated, dry-run canvas_transform plan.
// The plan is applied through the same gated path as every other live canvas write.
func BuildCardCreatePlan(opts CardCreateOptions) (TransformPlan, error) {
	if len(opts.Cards) == 0 {
		return TransformPlan{}, fmt.Errorf("canvas card create requires at least one card spec")
	}
	if len(opts.IDs) != len(opts.Cards) {
		return TransformPlan{}, fmt.Errorf("canvas card create requires one --id per card spec")
	}
	ops := make([]TransformOperation, 0, len(opts.Cards))
	for i, card := range opts.Cards {
		normalized, err := normalizeCardCreateSpec(strings.TrimSpace(opts.IDs[i]), card)
		if err != nil {
			return TransformPlan{}, err
		}
		ops = append(ops, TransformOperation{Kind: CardCreateKind, ID: strings.TrimSpace(opts.IDs[i]), After: normalized})
	}
	if err := validateTransformOperations(ops); err != nil {
		return TransformPlan{}, err
	}
	plan := TransformPlan{
		PlanType:     "canvas_transform",
		DryRun:       true,
		DocID:        opts.DocID,
		Source:       DiffSource{Mode: "direct", Count: len(ops)},
		AffectedIDs:  affectedIDs(ops),
		Operations:   ops,
		Integrity:    DocIntegrityResult{DocID: opts.DocID, OK: true, Summary: map[string]int{}},
		BackupTarget: opts.BackupTarget,
		Rollback:     TransformProof{Required: true, Fields: []string{"before_snapshot", "delta", "affected_ids"}},
		Proof:        TransformProof{Required: true, Fields: []string{"pre_integrity", "post_integrity", "reload_verification"}},
	}
	plan.PlanID = cardCreatePlanID(plan)
	return plan, nil
}

func cardCreatePlanID(plan TransformPlan) string {
	h := fnv.New32a()
	for _, op := range plan.Operations {
		raw, _ := json.Marshal(op)
		_, _ = h.Write(raw)
	}
	return fmt.Sprintf("canvas-card-create-%08x", h.Sum32())
}

// normalizeCardCreateSpec fills defaults that keep an AFFiNE note renderable
// and gives every leaf paragraph a deterministic block ID.
func normalizeCardCreateSpec(cardID string, card CardCreateSpec) (CardCreateSpec, error) {
	if card.Flavour == "" {
		card.Flavour = cardNoteFlavour
	}
	if card.DisplayMode == "" {
		card.DisplayMode = defaultCardDisplay
	}
	if card.Index == "" {
		card.Index = defaultCardIndex
	}
	if card.W == 0 {
		card.W = 360
	}
	if card.H == 0 {
		card.H = 220
	}
	paragraphs := make([]CardParagraph, 0, len(card.Paragraphs))
	for i, para := range card.Paragraphs {
		if strings.TrimSpace(para.ID) == "" {
			para.ID = fmt.Sprintf("%s-p%d", cardID, i)
		}
		if para.Type == "" {
			para.Type = "text"
		}
		paragraphs = append(paragraphs, para)
	}
	card.Paragraphs = paragraphs
	return card, nil
}

// decodeCardCreateSpec reads a card spec out of a plan operation. It works both for
// an in-memory plan and for a plan that round-tripped through JSON, and it rejects
// unknown fields so a malformed plan fails before any live apply.
func decodeCardCreateSpec(v any) (CardCreateSpec, error) {
	if spec, ok := v.(CardCreateSpec); ok {
		return spec, nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return CardCreateSpec{}, fmt.Errorf("create_card operation has an unreadable after payload")
	}
	var spec CardCreateSpec
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&spec); err != nil {
		return CardCreateSpec{}, fmt.Errorf("create_card operation has an invalid after payload: %w", err)
	}
	return spec, nil
}

// validateCardCreateOperations enforces every invariant a created AFFiNE note must
// carry before the Y.js write layer is allowed to touch a live document.
func validateCardCreateOperations(ops []TransformOperation) error {
	claimed := map[string]string{}
	for _, op := range ops {
		if op.Kind != CardCreateKind {
			continue
		}
		cardID := strings.TrimSpace(op.ID)
		if cardID == "" {
			return fmt.Errorf("create_card operation requires id")
		}
		if op.Before != nil {
			return fmt.Errorf("create_card operation for %q must not carry a before state", cardID)
		}
		spec, err := decodeCardCreateSpec(op.After)
		if err != nil {
			return fmt.Errorf("create_card operation for %q: %w", cardID, err)
		}
		if spec.Flavour != cardNoteFlavour {
			return fmt.Errorf("create_card operation for %q requires flavour %q, got %q", cardID, cardNoteFlavour, spec.Flavour)
		}
		if err := validateCardGeometry(cardID, spec); err != nil {
			return err
		}
		if !cardDisplayModes[spec.DisplayMode] {
			return fmt.Errorf("create_card operation for %q requires display_mode edgeless, both or page, got %q", cardID, spec.DisplayMode)
		}
		if strings.TrimSpace(spec.Index) == "" {
			return fmt.Errorf("create_card operation for %q requires index", cardID)
		}
		if !utf8.ValidString(spec.FrameID) || strings.TrimSpace(spec.FrameID) != spec.FrameID {
			return fmt.Errorf("create_card operation for %q has an invalid frame_id", cardID)
		}
		if len(spec.Paragraphs) == 0 {
			return fmt.Errorf("create_card operation for %q requires at least one paragraph block", cardID)
		}
		if err := claimBlockID(claimed, cardID, cardID); err != nil {
			return err
		}
		for _, para := range spec.Paragraphs {
			paraID := strings.TrimSpace(para.ID)
			if paraID == "" {
				return fmt.Errorf("create_card operation for %q requires an id on every paragraph block", cardID)
			}
			if !cardParagraphTypes[para.Type] {
				return fmt.Errorf("create_card operation for %q has unsupported paragraph type %q", cardID, para.Type)
			}
			if !utf8.ValidString(para.Text) {
				return fmt.Errorf("create_card operation for %q has paragraph %q with invalid UTF-8 text", cardID, paraID)
			}
			if err := claimBlockID(claimed, paraID, cardID); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateCardGeometry(cardID string, spec CardCreateSpec) error {
	for name, value := range map[string]float64{"x": spec.X, "y": spec.Y, "w": spec.W, "h": spec.H} {
		if value != value || value > 1e12 || value < -1e12 {
			return fmt.Errorf("create_card operation for %q has invalid %s", cardID, name)
		}
	}
	if spec.W <= 0 || spec.H <= 0 {
		return fmt.Errorf("create_card operation for %q requires positive w and h", cardID)
	}
	return nil
}

func claimBlockID(claimed map[string]string, id, owner string) error {
	if previous, ok := claimed[id]; ok {
		return fmt.Errorf("create_card plan reuses block id %q (already claimed by %q)", id, previous)
	}
	claimed[id] = owner
	return nil
}

// CardCreateDiffPreview summarizes create operations for the dry-run surface.
func CardCreateDiffPreview(ops []TransformOperation) []DiffIssue {
	issues := make([]DiffIssue, 0)
	for _, op := range ops {
		if op.Kind != CardCreateKind {
			continue
		}
		issues = append(issues, DiffIssue{
			ID:                  op.ID,
			After:               op.After,
			Category:            "card_created",
			Severity:            "info",
			SuggestedNextAction: "Review card geometry, frame and text before live apply.",
		})
	}
	return issues
}

// CardCreateIDs lists the block IDs a plan would create, card and leaf blocks alike.
func CardCreateIDs(ops []TransformOperation) []string {
	var ids []string
	for _, op := range ops {
		if op.Kind != CardCreateKind {
			continue
		}
		spec, err := decodeCardCreateSpec(op.After)
		if err != nil {
			continue
		}
		ids = append(ids, op.ID)
		for _, para := range spec.Paragraphs {
			ids = append(ids, para.ID)
		}
	}
	sort.Strings(ids)
	return ids
}

// ParseCardMarkdown turns operational card markdown into leaf paragraph blocks.
// Headings map to h1-h6, list markers to bulleted/numbered, fenced blocks to code.
func ParseCardMarkdown(markdown string) []CardParagraph {
	var out []CardParagraph
	var code []string
	inCode := false
	for _, rawLine := range strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n") {
		line := strings.TrimRight(rawLine, " \t\r")
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if inCode {
				out = append(out, CardParagraph{Type: "code", Text: strings.Join(code, "\n")})
				code = nil
			}
			inCode = !inCode
			continue
		}
		if inCode {
			code = append(code, line)
			continue
		}
		if trimmed == "" {
			continue
		}
		out = append(out, cardParagraphFromLine(trimmed))
	}
	if inCode && len(code) > 0 {
		out = append(out, CardParagraph{Type: "code", Text: strings.Join(code, "\n")})
	}
	return out
}

func cardParagraphFromLine(line string) CardParagraph {
	if level := strings.IndexFunc(line, func(r rune) bool { return r != '#' }); level > 0 && level <= 6 && line[level] == ' ' {
		return CardParagraph{Type: fmt.Sprintf("h%d", level), Text: strings.TrimSpace(line[level+1:])}
	}
	switch {
	case strings.HasPrefix(line, "- "), strings.HasPrefix(line, "* "):
		return CardParagraph{Type: "bulleted", Text: strings.TrimSpace(line[2:])}
	case strings.HasPrefix(line, "> "):
		return CardParagraph{Type: "quote", Text: strings.TrimSpace(line[2:])}
	}
	if dot := strings.Index(line, ". "); dot > 0 && dot <= 3 && isASCIIDigits(line[:dot]) {
		return CardParagraph{Type: "numbered", Text: strings.TrimSpace(line[dot+2:])}
	}
	return CardParagraph{Type: "text", Text: line}
}

func isASCIIDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

// asciiJSON marshals a value to JSON with every non-ASCII rune escaped as \uXXXX.
// The result is embedded in generated JavaScript, and pure-ASCII source removes the
// encoding round trip that produces mojibake in accented card text.
func asciiJSON(v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.Grow(len(raw))
	for _, r := range string(raw) {
		switch {
		case r < 0x80:
			b.WriteRune(r)
		case r > 0xFFFF:
			r -= 0x10000
			fmt.Fprintf(&b, "\\u%04x\\u%04x", 0xD800+(r>>10), 0xDC00+(r&0x3FF))
		default:
			fmt.Fprintf(&b, "\\u%04x", r)
		}
	}
	return b.String(), nil
}

// cardCreateScriptHelpers are the JavaScript helpers the create branch of the
// transform apply script relies on. They construct every AFFiNE structural field,
// most importantly sys:children as a real Y.Array on the note and on every leaf.
const cardCreateScriptHelpers = `
	function pageRootChildren() {
		var pageId = "";
		blocks.forEach(function(block, id) {
			if (!pageId && block instanceof Y.Map && block.get("sys:flavour") === "affine:page") pageId = id;
		});
		if (!pageId) throw new Error("page root not found");
		var page = blocks.get(pageId);
		var children = page.get("sys:children");
		if (!(children instanceof Y.Array)) throw new Error("page root has no sys:children Y.Array");
		return children;
	}
	function hasChildID(children, id) {
		for (var i = 0; i < children.length; i++) if (children.get(i) === id) return true;
		return false;
	}
	function toYMapField(owner, key) {
		var current = owner.get(key);
		if (current instanceof Y.Map) return current;
		var next = new Y.Map();
		if (current && typeof current === "object") {
			for (var k in current) next.set(k, current[k]);
		}
		owner.set(key, next);
		return next;
	}
	function surfaceElementValue() {
		var surface = null;
		blocks.forEach(function(block) {
			if (!surface && block instanceof Y.Map && block.get("sys:flavour") === "affine:surface") surface = block;
		});
		if (!(surface instanceof Y.Map)) return null;
		var raw = surface.get("prop:elements");
		if (raw instanceof Y.Map && raw.get("type") === "$blocksuite:internal:native$") {
			var value = raw.get("value");
			return value instanceof Y.Map ? value : null;
		}
		return raw instanceof Y.Map ? raw : null;
	}
	function attachToFrame(frameId, cardId) {
		var frameBlock = blocks.get(frameId);
		if (frameBlock instanceof Y.Map) {
			toYMapField(frameBlock, "prop:childElementIds").set(cardId, true);
			return "block";
		}
		var elements = surfaceElementValue();
		var element = elements ? elements.get(frameId) : null;
		if (element instanceof Y.Map) {
			toYMapField(element, "childElementIds").set(cardId, true);
			return "surface";
		}
		throw new Error("frame not found: " + frameId);
	}
	function createCard(cardId, spec) {
		if (blocks.get(cardId) instanceof Y.Map) throw new Error("block already exists: " + cardId);
		var note = new Y.Map();
		note.set("sys:id", cardId);
		note.set("sys:flavour", spec.flavour);
		note.set("sys:version", 1);
		var noteChildren = new Y.Array();
		note.set("sys:children", noteChildren);
		note.set("prop:xywh", xywhString([spec.x, spec.y, spec.w, spec.h]));
		note.set("prop:displayMode", spec.display_mode);
		note.set("prop:hidden", false);
		note.set("prop:index", spec.index);
		if (spec.background) note.set("prop:background", spec.background);
		blocks.set(cardId, note);
		var paragraphs = spec.paragraphs || [];
		for (var p = 0; p < paragraphs.length; p++) {
			var para = paragraphs[p];
			if (blocks.get(para.id) instanceof Y.Map) throw new Error("block already exists: " + para.id);
			var leaf = new Y.Map();
			leaf.set("sys:id", para.id);
			leaf.set("sys:flavour", "affine:paragraph");
			leaf.set("sys:version", 1);
			leaf.set("sys:children", new Y.Array());
			leaf.set("sys:type", para.type);
			leaf.set("prop:type", para.type);
			var text = new Y.Text();
			if (para.text) text.insert(0, para.text);
			leaf.set("prop:text", text);
			blocks.set(para.id, leaf);
			noteChildren.insert(noteChildren.length, [para.id]);
		}
		var pageChildren = pageRootChildren();
		if (!hasChildID(pageChildren, cardId)) pageChildren.insert(pageChildren.length, [cardId]);
		if (spec.frame_id) attachToFrame(spec.frame_id, cardId);
	}
`
