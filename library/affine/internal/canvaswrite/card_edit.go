package canvaswrite

import (
	"encoding/json"
	"fmt"
	"math"
)

// CardEditSpec replaces only explicitly named text leaves; media stays attached.
type CardEditSpec struct {
	Paragraphs       []CardParagraph   `json:"paragraphs,omitempty"`
	RemoveTextIDs    []string          `json:"remove_text_ids,omitempty"`
	ExpectedChildren []string          `json:"expected_children"`
	ExpectedTexts    map[string]string `json:"expected_texts,omitempty"`
	XYWH             []float64         `json:"xywh,omitempty"`
	Background       string            `json:"background,omitempty"`
	DisplayMode      string            `json:"display_mode,omitempty"`
	FrameID          string            `json:"frame_id,omitempty"`
	Image            *CardEditImage    `json:"image,omitempty"`
}

type CardEditImage struct {
	SourceID string `json:"source_id"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
}

type FrameEditSpec struct {
	Title      string    `json:"title"`
	XYWH       []float64 `json:"xywh"`
	Children   []string  `json:"children"`
	Background string    `json:"background,omitempty"`
	Index      string    `json:"index,omitempty"`
}

func decodeEdit[T any](v any) (T, error) {
	var out T
	raw, err := json.Marshal(v)
	if err == nil {
		err = json.Unmarshal(raw, &out)
	}
	return out, err
}

func validateEditGeometry(v []float64) error {
	if len(v) != 4 || v[2] <= 0 || v[3] <= 0 {
		return fmt.Errorf("geometry requires x,y,positive width,positive height")
	}
	for _, n := range v {
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return fmt.Errorf("geometry must be finite")
		}
	}
	return nil
}

func validateCanvasEditOperation(op TransformOperation) error {
	switch op.Kind {
	case "edit_card":
		s, err := decodeEdit[CardEditSpec](op.After)
		if err != nil {
			return err
		}
		if s.ExpectedChildren == nil {
			return fmt.Errorf("edit_card requires expected_children")
		}
		if s.Image != nil && (s.Image.SourceID == "" || s.Image.Width <= 0 || s.Image.Height <= 0) {
			return fmt.Errorf("image requires source_id and positive dimensions")
		}
		if len(s.XYWH) > 0 {
			if err := validateEditGeometry(s.XYWH); err != nil {
				return err
			}
		}
		if s.DisplayMode != "" && !cardDisplayModes[s.DisplayMode] {
			return fmt.Errorf("unsupported display mode")
		}
		seen := map[string]bool{}
		for _, p := range s.Paragraphs {
			if p.ID == "" || seen[p.ID] || !cardParagraphTypes[p.Type] {
				return fmt.Errorf("invalid or duplicate text paragraph %q", p.ID)
			}
			seen[p.ID] = true
		}
		for _, id := range s.RemoveTextIDs {
			if id == "" || seen[id] {
				return fmt.Errorf("invalid or conflicting text removal %q", id)
			}
			seen[id] = true
		}
	case "upsert_frame":
		s, err := decodeEdit[FrameEditSpec](op.After)
		if err != nil {
			return err
		}
		if s.Title == "" || s.Children == nil {
			return fmt.Errorf("frame requires title and children")
		}
		return validateEditGeometry(s.XYWH)
	case "remove_connector":
		if op.After != nil {
			return fmt.Errorf("remove_connector takes no after value")
		}
	}
	return nil
}

// BuildCanvasEditPlan validates a reviewable manifest before any network write.
func BuildCanvasEditPlan(docID string, ops []TransformOperation) (TransformPlan, error) {
	plan := TransformPlan{PlanType: "canvas_transform", DocID: docID, DryRun: true, Operations: ops,
		AffectedIDs: affectedIDs(ops), Integrity: DocIntegrityResult{DocID: docID, OK: true, Summary: map[string]int{}},
		Proof:    TransformProof{Required: true, Fields: []string{"pre_integrity", "post_integrity", "reload_verification"}},
		Rollback: TransformProof{Required: true, Fields: []string{"before_snapshot", "delta"}}}
	if docID == "" || len(ops) == 0 {
		return plan, fmt.Errorf("doc_id and operations are required")
	}
	seen := map[string]bool{}
	for _, op := range ops {
		if op.Kind != "edit_card" && op.Kind != "upsert_frame" && op.Kind != "remove_connector" {
			return plan, fmt.Errorf("unsupported edit operation %q", op.Kind)
		}
		if seen[op.ID] {
			return plan, fmt.Errorf("duplicate edit target %q", op.ID)
		}
		seen[op.ID] = true
	}
	if err := validateTransformOperations(ops); err != nil {
		return plan, err
	}
	plan.PlanID = transformPlanID(plan)
	return plan, nil
}

const canvasEditScriptHelpers = `
function editCanvas(op) {
 var s = op.after || {};
 if (op.kind === "remove_connector") {
  var elements = surfaceElementValue();
  var connector = elements && elements.get(op.id);
  if (!connector) throw new Error("connector not found: " + op.id);
  var type = connector instanceof Y.Map ? connector.get("type") : connector.type;
  if (type !== "connector") throw new Error("refusing non-connector removal: " + op.id);
  elements.delete(op.id);
  blocks.forEach(function(b) { if (b.get("sys:flavour") === "affine:frame") toYMapField(b,"prop:childElementIds").delete(op.id); });
  return;
 }
 if (op.kind === "upsert_frame") {
  var frame = blocks.get(op.id), created = !(frame instanceof Y.Map);
  if (!created && frame.get("sys:flavour") !== "affine:frame") throw new Error("target is not a frame");
  var surface;
  blocks.forEach(function(b) { if (b.get("sys:flavour") === "affine:surface") surface = b; });
  if (!surface) throw new Error("surface not found");
  for (var i=0; i<s.children.length; i++) if (!blocks.has(s.children[i]) && !(surfaceElementValue() && surfaceElementValue().has(s.children[i]))) throw new Error("frame child missing: " + s.children[i]);
  if (created) {
   frame = new Y.Map(); frame.set("sys:id",op.id); frame.set("sys:flavour","affine:frame"); frame.set("sys:version",1); frame.set("sys:children",new Y.Array()); blocks.set(op.id,frame);
   var sc=surface.get("sys:children"); if (!(sc instanceof Y.Array)) throw new Error("surface children missing"); sc.push([op.id]);
  }
  frame.set("prop:title",s.title); frame.set("prop:xywh",xywhString(s.xywh)); frame.set("prop:background",s.background || "transparent"); frame.set("prop:index",s.index || "a0");
  var members=new Y.Map(); s.children.forEach(function(id) { members.set(id,true); }); frame.set("prop:childElementIds",members);
  return;
 }
 var card=requireBlock(op.id);
 if (card.get("sys:flavour") !== "affine:note") throw new Error("target is not a card");
 var children=card.get("sys:children");
 if (!(children instanceof Y.Array)) throw new Error("card children missing");
 if (JSON.stringify(children.toArray()) !== JSON.stringify(s.expected_children)) throw new Error("stale card children: " + op.id);
 Object.keys(s.expected_texts || {}).forEach(function(id) { var b=blocks.get(id); var t=b && b.get("prop:text"); if (!b || String(t || "") !== s.expected_texts[id]) throw new Error("stale text: " + id); });
 var remove=[];
 function reviewRemoval(id) {
  var b=requireBlock(id), f=b.get("sys:flavour"), nested=b.get("sys:children");
  if (f !== "affine:paragraph" && f !== "affine:list") throw new Error("refusing non-text removal: " + id);
  if (nested && nested.length) nested.forEach(function(child) {
   if (!Object.prototype.hasOwnProperty.call(s.expected_texts || {},child)) throw new Error("unreviewed text child: " + child);
   reviewRemoval(child);
  });
  remove.push(id);
 }
 (s.remove_text_ids || []).forEach(function(id) {
  if (!hasChildID(children,id)) throw new Error("text removal outside card: " + id);
  reviewRemoval(id);
 });
 (s.paragraphs || []).forEach(function(p) {
  var b=blocks.get(p.id);
  if (b && (!hasChildID(children,p.id) || b.get("sys:flavour") !== "affine:paragraph")) throw new Error("text identity conflict: " + p.id);
 });
 (s.remove_text_ids || []).forEach(function(id) { children.delete(children.toArray().indexOf(id),1); });
 remove.forEach(function(id) { blocks.delete(id); });
 (s.paragraphs || []).forEach(function(p) {
  var b=blocks.get(p.id);
  if (!(b instanceof Y.Map)) { b=new Y.Map(); b.set("sys:id",p.id); b.set("sys:flavour","affine:paragraph"); b.set("sys:version",1); b.set("sys:children",new Y.Array()); blocks.set(p.id,b); children.push([p.id]); }
  b.set("sys:type",p.type); b.set("prop:type",p.type);
  var t=b.get("prop:text"); if (!(t instanceof Y.Text)) { t=new Y.Text(); b.set("prop:text",t); }
  if (t.length) t.delete(0,t.length);
  var pos=0; parseInlineMarkdown(p.text).forEach(function(seg) { t.insert(pos,seg.text,seg.attrs || {}); pos+=seg.text.length; });
 });
 if (s.image) {
  var image;
  children.forEach(function(id) { var b=blocks.get(id); if (!image && b && b.get("sys:flavour") === "affine:image") image=b; });
  if (!image) {
   var id=op.id+"-image"; if (blocks.has(id)) throw new Error("image identity conflict: " + id);
   image=new Y.Map(); image.set("sys:id",id); image.set("sys:flavour","affine:image"); image.set("sys:version",1); image.set("sys:children",new Y.Array()); blocks.set(id,image); children.insert(0,[id]);
  }
  image.set("prop:sourceId",s.image.source_id); image.set("prop:width",s.image.width); image.set("prop:height",s.image.height); image.set("prop:rotate",0); image.set("prop:size",-1); image.set("prop:caption","");
 }
 if (s.xywh) card.set("prop:xywh",xywhString(s.xywh));
 if (s.background) card.set("prop:background",s.background);
 if (s.display_mode) card.set("prop:displayMode",s.display_mode);
 if (s.frame_id) {
  blocks.forEach(function(b) { if (b.get("sys:flavour") === "affine:frame") toYMapField(b,"prop:childElementIds").delete(op.id); });
  attachToFrame(s.frame_id,op.id);
 }
}
`
