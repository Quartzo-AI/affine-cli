# Verification: Phase 8 Workflow Proof, Docs, And Publish Durability

## Required Checks

- Full package tests from `library/affine`.
- CLI help output for changed commands.
- Patch record exists under `library/affine/.printing-press-patches/`.
- Workflow proof includes connector-label planning and dry-run apply.

## Evidence To Record

- Test commands and results.
- Help command outputs checked.
- Patch record path.
- Any publish validation limitation.

## Evidence Recorded

- `go test ./internal/canvaswrite ./internal/cli` passed.
- `go test ./...` passed.
- Help smoke passed: `go run ./cmd/affine-pp-cli canvas label-plan --help`.
- Discovery smoke passed: `go run ./cmd/affine-pp-cli which "connector label" --json`.
- Dry-run apply smoke passed with stdin `canvas_connector_labels` plan.
- Patch record added: `library/affine/.printing-press-patches/affine-canvas-connector-label-writing.json`.
- Publish validation not run in this pass; no release or publish was requested.
