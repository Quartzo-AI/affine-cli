package canvaswrite

import (
	"fmt"
	"strings"
	"testing"
)

func TestEditPreservesMediaIdentityAndFormatsText(t *testing.T) {
	e, d := newCardTestDoc(t)
	plan, err := BuildCardCreatePlan(sampleCardOptions())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = e.RunScript(transformApplyScript(d, plan.Operations)); err != nil {
		t.Fatal(err)
	}
	_, err = e.RunScript(fmt.Sprintf(`var b=globalThis._docs[%d].getMap("blocks"); var m=new Y.Map(); m.set("sys:flavour","affine:image"); m.set("sys:children",new Y.Array()); m.set("prop:sourceId","original"); b.set("media",m); b.get("card-a").get("sys:children").push(["media"]); "ok";`, d))
	if err != nil {
		t.Fatal(err)
	}
	spec := CardEditSpec{ExpectedChildren: []string{"card-a-p0", "card-a-p1", "media"}, Paragraphs: []CardParagraph{{ID: "card-a-p0", Type: "h3", Text: "Operação"}, {ID: "card-a-p1", Type: "text", Text: "**Função:** [Operar](https://example.com)"}}}
	ops := []TransformOperation{{Kind: "edit_card", ID: "card-a", After: spec}}
	if _, err = BuildCanvasEditPlan("doc", ops); err != nil {
		t.Fatal(err)
	}
	if _, err = e.RunScript(transformApplyScript(d, ops)); err != nil {
		t.Fatal(err)
	}
	got, err := e.RunScript(fmt.Sprintf(`JSON.stringify({children:globalThis._docs[%d].getMap("blocks").get("card-a").get("sys:children").toArray(),media:globalThis._docs[%d].getMap("blocks").get("media").get("prop:sourceId"),delta:globalThis._docs[%d].getMap("blocks").get("card-a-p1").get("prop:text").toDelta()})`, d, d, d))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `"media":"original"`) || !strings.Contains(got, `"bold":true`) || !strings.Contains(got, `"link":"https://example.com"`) {
		t.Fatal(got)
	}
	spec.ExpectedTexts = map[string]string{"card-a-p0": "Changed elsewhere"}
	ops[0].After = spec
	if _, err = e.RunScript(transformApplyScript(d, ops)); err == nil {
		t.Fatal("stale text accepted")
	}
	spec.ExpectedTexts = nil
	spec.ExpectedChildren = []string{"wrong"}
	ops[0].After = spec
	if _, err = e.RunScript(transformApplyScript(d, ops)); err == nil {
		t.Fatal("stale manifest accepted")
	}
	spec.ExpectedChildren = []string{"card-a-p0", "card-a-p1", "media"}
	spec.RemoveTextIDs = []string{"media"}
	ops[0].After = spec
	if _, err = e.RunScript(transformApplyScript(d, ops)); err == nil {
		t.Fatal("media deletion accepted")
	}
}

func TestEditFrameAndRemoveOnlyConnector(t *testing.T) {
	e, d := newCardTestDoc(t)
	plan, _ := BuildCardCreatePlan(sampleCardOptions())
	if _, err := e.RunScript(transformApplyScript(d, plan.Operations)); err != nil {
		t.Fatal(err)
	}
	ops := []TransformOperation{{Kind: "upsert_frame", ID: "new-frame", After: FrameEditSpec{Title: "Construir", XYWH: []float64{0, 0, 1000, 800}, Children: []string{"card-a"}}}}
	if _, err := e.RunScript(transformApplyScript(d, ops)); err != nil {
		t.Fatal(err)
	}
	b, err := e.ReadBlocks(d)
	if err != nil {
		t.Fatal(err)
	}
	if !CheckBlocksIntegrity("doc", b).OK {
		t.Fatal("frame breaks integrity")
	}
	_, err = e.RunScript(fmt.Sprintf(`var els=globalThis._docs[%d].getMap("blocks").get("surface-1").get("prop:elements").get("value"); var c=new Y.Map(); c.set("type","connector"); els.set("line",c); "ok";`, d))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = e.RunScript(transformApplyScript(d, []TransformOperation{{Kind: "remove_connector", ID: "frame-1"}})); err == nil {
		t.Fatal("frame deletion accepted")
	}
	if _, err = e.RunScript(transformApplyScript(d, []TransformOperation{{Kind: "remove_connector", ID: "line"}})); err != nil {
		t.Fatal(err)
	}
}

func TestEditRequiresReviewedTextSubtree(t *testing.T) {
	e, d := newCardTestDoc(t)
	plan, _ := BuildCardCreatePlan(sampleCardOptions())
	if _, err := e.RunScript(transformApplyScript(d, plan.Operations)); err != nil {
		t.Fatal(err)
	}
	_, err := e.RunScript(fmt.Sprintf(`var b=globalThis._docs[%d].getMap("blocks");var c=new Y.Map();c.set("sys:flavour","affine:paragraph");c.set("sys:children",new Y.Array());c.set("prop:text",new Y.Text("Nested"));b.set("nested",c);b.get("card-a-p1").get("sys:children").push(["nested"]);"ok";`, d))
	if err != nil {
		t.Fatal(err)
	}
	spec := CardEditSpec{ExpectedChildren: []string{"card-a-p0", "card-a-p1"}, RemoveTextIDs: []string{"card-a-p1"}}
	ops := []TransformOperation{{Kind: "edit_card", ID: "card-a", After: spec}}
	if _, err = e.RunScript(transformApplyScript(d, ops)); err == nil {
		t.Fatal("unreviewed subtree accepted")
	}
	spec.ExpectedTexts = map[string]string{"nested": "Nested"}
	ops[0].After = spec
	if _, err = e.RunScript(transformApplyScript(d, ops)); err != nil {
		t.Fatal(err)
	}
	blocks, err := e.ReadBlocks(d)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := blocks["nested"]; exists {
		t.Fatal("orphan retained")
	}
	if !CheckBlocksIntegrity("doc", blocks).OK {
		t.Fatal("integrity failed")
	}
}
