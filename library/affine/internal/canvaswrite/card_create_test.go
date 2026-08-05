package canvaswrite

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"affine-pp-cli/internal/yjs"
)

func sampleCardOptions() CardCreateOptions {
	return CardCreateOptions{
		DocID: "doc-1",
		IDs:   []string{"card-a"},
		Cards: []CardCreateSpec{{
			X: 10, Y: 20, W: 360, H: 220,
			FrameID: "frame-1",
			Paragraphs: []CardParagraph{
				{Type: "h3", Text: "Card A"},
				{Type: "text", Text: "Body"},
			},
		}},
	}
}

func TestBuildCardCreatePlanAppliesDefaultsAndIDs(t *testing.T) {
	plan, err := BuildCardCreatePlan(sampleCardOptions())
	if err != nil {
		t.Fatalf("BuildCardCreatePlan error: %v", err)
	}
	if plan.PlanType != "canvas_transform" || !plan.DryRun {
		t.Fatalf("plan header = %#v, want dry-run canvas_transform", plan)
	}
	if !strings.HasPrefix(plan.PlanID, "canvas-card-create-") {
		t.Fatalf("plan_id = %q, want canvas-card-create- prefix", plan.PlanID)
	}
	if len(plan.Operations) != 1 || plan.Operations[0].Kind != CardCreateKind {
		t.Fatalf("operations = %#v, want one create_card operation", plan.Operations)
	}
	spec, err := decodeCardCreateSpec(plan.Operations[0].After)
	if err != nil {
		t.Fatalf("decodeCardCreateSpec error: %v", err)
	}
	if spec.Flavour != "affine:note" || spec.DisplayMode != "edgeless" || spec.Index != "a0" {
		t.Fatalf("spec defaults = %#v, want affine:note/edgeless/a0", spec)
	}
	if spec.Paragraphs[0].ID != "card-a-p0" || spec.Paragraphs[1].ID != "card-a-p1" {
		t.Fatalf("paragraph ids = %#v, want deterministic per-card ids", spec.Paragraphs)
	}
	want := []string{"card-a", "card-a-p0", "card-a-p1"}
	if got := CardCreateIDs(plan.Operations); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("CardCreateIDs = %v, want %v", got, want)
	}
}

func TestBuildCardCreatePlanRequiresCardsAndIDs(t *testing.T) {
	if _, err := BuildCardCreatePlan(CardCreateOptions{DocID: "doc-1"}); err == nil {
		t.Fatal("empty plan accepted, want error")
	}
	opts := sampleCardOptions()
	opts.IDs = nil
	if _, err := BuildCardCreatePlan(opts); err == nil {
		t.Fatal("card without id accepted, want error")
	}
}

func TestValidateCardCreateRejectsMissingInvariants(t *testing.T) {
	base := func() map[string]any {
		return map[string]any{
			"flavour":      "affine:note",
			"x":            0.0,
			"y":            0.0,
			"w":            360.0,
			"h":            220.0,
			"display_mode": "edgeless",
			"index":        "a0",
			"paragraphs":   []any{map[string]any{"id": "card-a-p0", "type": "text", "text": "Body"}},
		}
	}
	cases := []struct {
		name   string
		id     string
		mutate func(map[string]any)
	}{
		{name: "missing id", id: "", mutate: func(map[string]any) {}},
		{name: "wrong flavour", id: "card-a", mutate: func(m map[string]any) { m["flavour"] = "affine:paragraph" }},
		{name: "missing flavour", id: "card-a", mutate: func(m map[string]any) { delete(m, "flavour") }},
		{name: "zero width", id: "card-a", mutate: func(m map[string]any) { m["w"] = 0.0 }},
		{name: "negative height", id: "card-a", mutate: func(m map[string]any) { m["h"] = -5.0 }},
		{name: "missing display mode", id: "card-a", mutate: func(m map[string]any) { delete(m, "display_mode") }},
		{name: "unknown display mode", id: "card-a", mutate: func(m map[string]any) { m["display_mode"] = "floating" }},
		{name: "missing index", id: "card-a", mutate: func(m map[string]any) { m["index"] = "" }},
		{name: "no paragraphs", id: "card-a", mutate: func(m map[string]any) { m["paragraphs"] = []any{} }},
		{name: "paragraph without id", id: "card-a", mutate: func(m map[string]any) {
			m["paragraphs"] = []any{map[string]any{"type": "text", "text": "Body"}}
		}},
		{name: "paragraph with unsupported type", id: "card-a", mutate: func(m map[string]any) {
			m["paragraphs"] = []any{map[string]any{"id": "p0", "type": "callout", "text": "Body"}}
		}},
		{name: "paragraph id collides with card", id: "card-a", mutate: func(m map[string]any) {
			m["paragraphs"] = []any{map[string]any{"id": "card-a", "type": "text", "text": "Body"}}
		}},
		{name: "unknown spec field", id: "card-a", mutate: func(m map[string]any) { m["children"] = []any{} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			after := base()
			tc.mutate(after)
			ops := []TransformOperation{{Kind: CardCreateKind, ID: tc.id, After: after}}
			if err := validateTransformOperations(ops); err == nil {
				t.Fatalf("malformed create plan accepted (%s)", tc.name)
			}
		})
	}
}

func TestValidateCardCreateRejectsDuplicateIDsAcrossCards(t *testing.T) {
	opts := sampleCardOptions()
	opts.IDs = []string{"card-a", "card-a"}
	opts.Cards = append(opts.Cards, opts.Cards[0])
	if _, err := BuildCardCreatePlan(opts); err == nil {
		t.Fatal("duplicate card ids accepted, want error")
	}
}

func TestValidateCardCreateRejectsBeforeState(t *testing.T) {
	plan, err := BuildCardCreatePlan(sampleCardOptions())
	if err != nil {
		t.Fatalf("BuildCardCreatePlan error: %v", err)
	}
	plan.Operations[0].Before = map[string]any{"prop:xywh": "[0,0,1,1]"}
	if err := validateTransformOperations(plan.Operations); err == nil {
		t.Fatal("create_card with before state accepted, want error")
	}
}

func TestValidateTransformApplyGatesCardCreatePlan(t *testing.T) {
	plan, err := BuildCardCreatePlan(sampleCardOptions())
	if err != nil {
		t.Fatalf("BuildCardCreatePlan error: %v", err)
	}
	for _, opts := range []TransformApplyOptions{
		{DocID: "doc-1", BackupDir: "d"},
		{WorkspaceID: "ws", BackupDir: "d"},
		{WorkspaceID: "ws", DocID: "doc-1"},
		{WorkspaceID: "ws", DocID: "other-doc", BackupDir: "d"},
	} {
		if err := ValidateTransformApply(plan, opts); err == nil {
			t.Fatalf("live apply accepted without full gates: %#v", opts)
		}
	}
	if err := ValidateTransformApply(plan, TransformApplyOptions{WorkspaceID: "ws", DocID: "doc-1", BackupDir: "d"}); err != nil {
		t.Fatalf("fully gated apply rejected: %v", err)
	}
}

// newCardTestDoc builds a minimal edgeless document: one page root, one surface
// child, and one frame element inside the surface.
func newCardTestDoc(t *testing.T) (*yjs.Engine, int) {
	t.Helper()
	engine, err := yjs.NewEngine()
	if err != nil {
		t.Fatalf("NewEngine error: %v", err)
	}
	doc, err := engine.NewDoc()
	if err != nil {
		t.Fatalf("NewDoc error: %v", err)
	}
	if _, err := engine.RunScript(fmt.Sprintf(`
		(function() {
			var doc = globalThis._docs[%d];
			var blocks = doc.getMap("blocks");
			var page = new Y.Map();
			page.set("sys:id", "page-1");
			page.set("sys:flavour", "affine:page");
			page.set("sys:version", 1);
			var pageChildren = new Y.Array();
			page.set("sys:children", pageChildren);
			blocks.set("page-1", page);

			var surface = new Y.Map();
			surface.set("sys:id", "surface-1");
			surface.set("sys:flavour", "affine:surface");
			surface.set("sys:version", 1);
			surface.set("sys:children", new Y.Array());
			var boxed = new Y.Map();
			boxed.set("type", "$blocksuite:internal:native$");
			var elements = new Y.Map();
			var frame = new Y.Map();
			frame.set("id", "frame-1");
			frame.set("type", "frame");
			elements.set("frame-1", frame);
			boxed.set("value", elements);
			surface.set("prop:elements", boxed);
			blocks.set("surface-1", surface);
			pageChildren.insert(0, ["surface-1"]);
			return "ok";
		})()
	`, doc)); err != nil {
		t.Fatalf("seed doc error: %v", err)
	}
	return engine, doc
}

func TestTransformApplyScriptCreatesCardWithRequiredInvariants(t *testing.T) {
	engine, doc := newCardTestDoc(t)
	plan, err := BuildCardCreatePlan(sampleCardOptions())
	if err != nil {
		t.Fatalf("BuildCardCreatePlan error: %v", err)
	}
	if _, err := engine.RunScript(transformApplyScript(doc, plan.Operations)); err != nil {
		t.Fatalf("create_card apply error: %v", err)
	}
	blocks, err := engine.ReadBlocks(doc)
	if err != nil {
		t.Fatalf("ReadBlocks error: %v", err)
	}
	card := blocks["card-a"]
	if card == nil {
		t.Fatal("card block was not created")
	}
	if card["sys:flavour"] != "affine:note" {
		t.Fatalf("sys:flavour = %v, want affine:note", card["sys:flavour"])
	}
	if card["prop:xywh"] != "[10,20,360,220]" {
		t.Fatalf("prop:xywh = %v, want [10,20,360,220]", card["prop:xywh"])
	}
	if card["prop:displayMode"] != "edgeless" {
		t.Fatalf("prop:displayMode = %v, want edgeless", card["prop:displayMode"])
	}
	children, ok := card["sys:children"].([]any)
	if !ok || len(children) != 2 || children[0] != "card-a-p0" {
		t.Fatalf("card sys:children = %#v, want the two created leaf ids", card["sys:children"])
	}
	leaf := blocks["card-a-p0"]
	if leaf == nil || leaf["sys:flavour"] != "affine:paragraph" || leaf["prop:type"] != "h3" || leaf["prop:text"] != "Card A" {
		t.Fatalf("leaf block = %#v, want an affine:paragraph h3 with its text", leaf)
	}
	pageChildren, _ := blocks["page-1"]["sys:children"].([]any)
	if len(pageChildren) != 2 || pageChildren[1] != "card-a" {
		t.Fatalf("page sys:children = %#v, want the card appended", pageChildren)
	}
	if result := CheckBlocksIntegrity("doc-1", blocks); !result.OK {
		t.Fatalf("integrity after create = %#v, want OK", result)
	}
}

func TestTransformApplyScriptCreatesYArrayChildrenEverywhere(t *testing.T) {
	engine, doc := newCardTestDoc(t)
	plan, err := BuildCardCreatePlan(sampleCardOptions())
	if err != nil {
		t.Fatalf("BuildCardCreatePlan error: %v", err)
	}
	if _, err := engine.RunScript(transformApplyScript(doc, plan.Operations)); err != nil {
		t.Fatalf("create_card apply error: %v", err)
	}
	// A plain JS array reads back from ReadBlocks exactly like a Y.Array, so the
	// Y.Array invariant is only provable inside the runtime.
	for _, id := range CardCreateIDs(plan.Operations) {
		got, err := engine.RunScript(fmt.Sprintf(`
			(function() {
				var blocks = globalThis._docs[%d].getMap("blocks");
				var block = blocks.get(%q);
				if (!(block instanceof Y.Map)) return "missing block";
				return String(block.get("sys:children") instanceof Y.Array);
			})()
		`, doc, id))
		if err != nil {
			t.Fatalf("probe %s error: %v", id, err)
		}
		if got != "true" {
			t.Fatalf("block %s sys:children is not a Y.Array (probe = %q)", id, got)
		}
	}
}

func TestTransformApplyScriptRegistersCardInFrame(t *testing.T) {
	engine, doc := newCardTestDoc(t)
	plan, err := BuildCardCreatePlan(sampleCardOptions())
	if err != nil {
		t.Fatalf("BuildCardCreatePlan error: %v", err)
	}
	if _, err := engine.RunScript(transformApplyScript(doc, plan.Operations)); err != nil {
		t.Fatalf("create_card apply error: %v", err)
	}
	got, err := engine.RunScript(fmt.Sprintf(`
		(function() {
			var blocks = globalThis._docs[%d].getMap("blocks");
			var elements = blocks.get("surface-1").get("prop:elements").get("value");
			var ids = elements.get("frame-1").get("childElementIds");
			return String(ids instanceof Y.Map) + ":" + String(ids.get("card-a"));
		})()
	`, doc))
	if err != nil {
		t.Fatalf("frame probe error: %v", err)
	}
	if got != "true:true" {
		t.Fatalf("frame childElementIds probe = %q, want true:true", got)
	}
}

func TestTransformApplyScriptCreateCardMissingFrameFails(t *testing.T) {
	engine, doc := newCardTestDoc(t)
	opts := sampleCardOptions()
	opts.Cards[0].FrameID = "frame-absent"
	plan, err := BuildCardCreatePlan(opts)
	if err != nil {
		t.Fatalf("BuildCardCreatePlan error: %v", err)
	}
	if _, err := engine.RunScript(transformApplyScript(doc, plan.Operations)); err == nil {
		t.Fatal("create against a missing frame succeeded, want error")
	}
}

func TestTransformApplyScriptCreateCardRejectsExistingBlockID(t *testing.T) {
	engine, doc := newCardTestDoc(t)
	opts := sampleCardOptions()
	opts.IDs = []string{"surface-1"}
	plan, err := BuildCardCreatePlan(opts)
	if err != nil {
		t.Fatalf("BuildCardCreatePlan error: %v", err)
	}
	if _, err := engine.RunScript(transformApplyScript(doc, plan.Operations)); err == nil {
		t.Fatal("create over an existing block id succeeded, want error")
	}
}

func TestTransformApplyScriptCreateCardPreservesUTF8(t *testing.T) {
	engine, doc := newCardTestDoc(t)
	accented := "Configuração — não há ação pendente ✅"
	opts := sampleCardOptions()
	opts.Cards[0].Paragraphs = []CardParagraph{{Type: "h3", Text: accented}}
	plan, err := BuildCardCreatePlan(opts)
	if err != nil {
		t.Fatalf("BuildCardCreatePlan error: %v", err)
	}
	script := transformApplyScript(doc, plan.Operations)
	for _, r := range script {
		if r > 127 {
			t.Fatalf("generated script carries a non-ASCII rune %q; card text must be escaped", r)
		}
	}
	if _, err := engine.RunScript(script); err != nil {
		t.Fatalf("create_card apply error: %v", err)
	}
	blocks, err := engine.ReadBlocks(doc)
	if err != nil {
		t.Fatalf("ReadBlocks error: %v", err)
	}
	if got := blocks["card-a-p0"]["prop:text"]; got != accented {
		t.Fatalf("prop:text = %q, want %q", got, accented)
	}
	for _, mojibake := range []string{"??", "Ã", "ï¿½"} {
		if strings.Contains(fmt.Sprint(blocks["card-a-p0"]["prop:text"]), mojibake) {
			t.Fatalf("card text shows mojibake %q", mojibake)
		}
	}
}

func TestAsciiJSONEscapesNonASCII(t *testing.T) {
	out, err := asciiJSON(map[string]string{"text": "ação 🌍"})
	if err != nil {
		t.Fatalf("asciiJSON error: %v", err)
	}
	for _, r := range out {
		if r > 127 {
			t.Fatalf("asciiJSON output has non-ASCII rune %q", r)
		}
	}
	var back map[string]string
	if err := json.Unmarshal([]byte(out), &back); err != nil {
		t.Fatalf("asciiJSON output is not JSON: %v", err)
	}
	if back["text"] != "ação 🌍" {
		t.Fatalf("round trip = %q, want the original text", back["text"])
	}
}

func TestParseCardMarkdown(t *testing.T) {
	paragraphs := ParseCardMarkdown("### Card name\n\n##### Description\nShort sentence.\n- item\n> quoted\n2. second\n```\nrun me\n```\n")
	want := []CardParagraph{
		{Type: "h3", Text: "Card name"},
		{Type: "h5", Text: "Description"},
		{Type: "text", Text: "Short sentence."},
		{Type: "bulleted", Text: "item"},
		{Type: "quote", Text: "quoted"},
		{Type: "numbered", Text: "second"},
		{Type: "code", Text: "run me"},
	}
	if fmt.Sprint(paragraphs) != fmt.Sprint(want) {
		t.Fatalf("ParseCardMarkdown = %#v, want %#v", paragraphs, want)
	}
}
