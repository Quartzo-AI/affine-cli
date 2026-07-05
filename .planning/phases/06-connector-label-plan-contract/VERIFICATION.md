# Verification: Phase 6 Connector Label Plan Contract

## Required Checks

- Unit tests for plan generation from connector IDs.
- Unit tests for batch JSON input.
- Unit tests for duplicate and ambiguous updates.
- Dry-run compatibility test through `canvas apply --dry-run`.

## Evidence To Record

- Example review plan with at least three connector label updates.
- Proof that the planner command is marked read-only.
- Proof that the apply path treats the plan as a dry-run unless explicit live gates are supplied.

## Evidence Recorded

- `go test ./internal/canvaswrite ./internal/cli` passed.
- `go test ./...` passed.
- `canvas label-plan` is annotated with `mcp:read-only=true`.
- `canvas apply --dry-run --json` accepts `plan_type: canvas_connector_labels` and returns `live_write_supported: true` plus required live gates instead of mutating.
- CLI smoke passed: `go run ./cmd/affine-pp-cli canvas label-plan --help`.
- Discovery smoke passed: `go run ./cmd/affine-pp-cli which "connector label" --json`.
