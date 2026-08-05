package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func runCanvasCardCreate(t *testing.T, args ...string) (map[string]any, string, error) {
	t.Helper()
	root := RootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetIn(strings.NewReader(""))
	root.SetArgs(args)
	err := root.Execute()
	if err != nil {
		return nil, out.String(), err
	}
	var got map[string]any
	if jsonErr := json.Unmarshal(out.Bytes(), &got); jsonErr != nil {
		t.Fatalf("output is not JSON: %v\n%s", jsonErr, out.String())
	}
	return got, out.String(), nil
}

func TestRootIncludesCanvasCardCreate(t *testing.T) {
	cmd, _, err := RootCmd().Find([]string{"canvas", "card", "create"})
	if err != nil || cmd == nil || cmd.Name() != "create" {
		t.Fatalf("Find(canvas card create) = %v, %v", cmd, err)
	}
}

// A create without --apply/--live must produce only a plan: no workspace, no
// backup dir, and no live connection are required or used.
func TestCanvasCardCreateDryRunEmitsPlanOnly(t *testing.T) {
	got, raw, err := runCanvasCardCreate(t,
		"canvas", "card", "create",
		"--doc", "doc-1",
		"--id", "card-a",
		"--x", "10", "--y", "20", "--w", "360", "--h", "220",
		"--frame", "frame-1",
		"--text", "### Configuração\\n\\nNão há ação pendente.",
		"--json",
	)
	if err != nil {
		t.Fatalf("Execute error: %v\n%s", err, raw)
	}
	if got["dry_run"] != true || got["plan_type"] != "canvas_transform" {
		t.Fatalf("dry-run output = %#v, want a dry-run canvas_transform plan", got)
	}
	created, _ := got["created_block_ids"].([]any)
	if len(created) != 3 {
		t.Fatalf("created_block_ids = %#v, want the card plus two leaf blocks", got["created_block_ids"])
	}
	ops, _ := got["operations"].([]any)
	if len(ops) != 1 {
		t.Fatalf("operations = %#v, want one create operation", got["operations"])
	}
	op, _ := ops[0].(map[string]any)
	if op["kind"] != "create_card" || op["id"] != "card-a" {
		t.Fatalf("operation = %#v, want create_card for card-a", op)
	}
	after, _ := op["after"].(map[string]any)
	if after["flavour"] != "affine:note" || after["display_mode"] != "edgeless" || after["frame_id"] != "frame-1" {
		t.Fatalf("card spec = %#v, want the note invariants carried in the plan", after)
	}
	paragraphs, _ := after["paragraphs"].([]any)
	if len(paragraphs) != 2 {
		t.Fatalf("paragraphs = %#v, want heading plus body", after["paragraphs"])
	}
	head, _ := paragraphs[0].(map[string]any)
	body, _ := paragraphs[1].(map[string]any)
	if head["type"] != "h3" || head["text"] != "Configuração" {
		t.Fatalf("heading paragraph = %#v, want an h3 with intact accents", head)
	}
	if body["text"] != "Não há ação pendente." {
		t.Fatalf("body text = %q, want intact UTF-8", body["text"])
	}
}

func TestCanvasCardCreateLiveRequiresConfirmation(t *testing.T) {
	_, raw, err := runCanvasCardCreate(t,
		"canvas", "card", "create",
		"--doc", "doc-1", "--id", "card-a", "--text", "### Card",
		"--live", "--workspace", "ws", "--backup-dir", "./backups",
		"--json",
	)
	if err == nil {
		t.Fatalf("live apply without --yes succeeded, want confirmation error\n%s", raw)
	}
	if !strings.Contains(err.Error(), "confirmation required") {
		t.Fatalf("error = %v, want a confirmation gate error", err)
	}
}

func TestCanvasCardCreateLiveRequiresBackupDir(t *testing.T) {
	_, raw, err := runCanvasCardCreate(t,
		"canvas", "card", "create",
		"--doc", "doc-1", "--id", "card-a", "--text", "### Card",
		"--live", "--workspace", "ws", "--yes",
		"--json",
	)
	if err == nil {
		t.Fatalf("live apply without --backup-dir succeeded, want gate error\n%s", raw)
	}
	if !strings.Contains(err.Error(), "backup-dir") {
		t.Fatalf("error = %v, want a backup-dir gate error", err)
	}
}

func TestCanvasCardCreateRejectsCardWithoutText(t *testing.T) {
	if _, raw, err := runCanvasCardCreate(t, "canvas", "card", "create", "--doc", "doc-1", "--id", "card-a", "--json"); err == nil {
		t.Fatalf("card without text accepted, want error\n%s", raw)
	}
}

func TestCanvasCardCreateReadsSpecFromStdin(t *testing.T) {
	root := RootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetIn(strings.NewReader(`{"doc_id":"doc-1","cards":[
		{"id":"card-a","x":0,"y":0,"w":360,"h":220,"markdown":"### A\nBody"},
		{"id":"card-b","x":400,"y":0,"w":360,"h":220,"markdown":"### B\nBody"}
	]}`))
	root.SetArgs([]string{"canvas", "card", "create", "--spec", "-", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute error: %v\n%s", err, out.String())
	}
	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out.String())
	}
	affected, _ := got["affected_ids"].([]any)
	if len(affected) != 2 {
		t.Fatalf("affected_ids = %#v, want both cards", got["affected_ids"])
	}
	if got["doc_id"] != "doc-1" {
		t.Fatalf("doc_id = %v, want doc-1 from the spec file", got["doc_id"])
	}
}
