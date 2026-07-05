# State

## Current Status

- Active milestone: v1.2 Connector Label Writing
- Current phase: v1.2 connector label writing applied and verified
- Last planning update: 2026-06-20
- Git baseline: main at 4f9a09c3

## Decisions

- Use GSD planning in `.planning/` for the AFFiNE CLI repo.
- Execute milestone phases serially because search/model semantics are dependencies for diff, transform, and live apply.
- Use at most two active subagents per wave.
- Keep all Canvas mutations dry-run by default. Live apply requires explicit apply gates and verification.
- Connector label writing must be review-first: no label is inserted into an AFFiNE document until the operator approves the planned insertions.

## Completed

- Phase 1 implemented `canvas search` for read-only block/card/connector selection.
- Snapshot-file mode no longer requires live `--workspace`/`--doc` flags for audit, integrity, or search.
- Verification passed: `go test ./internal/canvaswrite ./internal/cli`, `go test ./...`, `go run ./cmd/affine-pp-cli canvas search --help`, and `go run ./cmd/affine-pp-cli which canvas --json`.
- Phase 2 implemented `canvas diff` for read-only semantic comparison across snapshot, history and live source modes.
- Verification passed: `go test ./...`, `go run ./cmd/affine-pp-cli canvas diff --help`, and `go run ./cmd/affine-pp-cli which "canvas diff" --json`.
- Phase 3 implemented `canvas transform` dry-run operation plans and transform-plan compatibility in `canvas apply --dry-run`.
- Verification passed: `go test ./internal/canvaswrite ./internal/cli`, `go test ./...`, `go run ./cmd/affine-pp-cli canvas transform --help`, `go run ./cmd/affine-pp-cli which "canvas transform" --json`, and selector-to-apply dry-run smoke.
- Phase 4 implemented gated live apply for transform and layout Canvas plans with required `--live` or `--apply`, `--workspace`, `--doc`, `--backup-dir`, and `--yes`; semantic diff preview, backups, pre/post integrity, tests, live fixture smoke, and workflow proof passed.
- Milestone v1.1 completed with accepted tech debt documented in `.planning/v1.1-MILESTONE-AUDIT.md`.
- Milestone v1.2 planning artifacts created for Connector Label Writing.
- Phase 5 implemented connector label extraction in canvas search/model surfaces and `connector_label_changed` semantic diff reporting.
- Phase 6 implemented `canvas label-plan` for reviewable connector label plans and `canvas apply --dry-run` compatibility for `canvas_connector_labels`.
- Phase 7 implemented gated connector label live apply path with backup, integrity, source/target preservation, and post-reload label verification. Live insertion into the AFFiNE board is pending operator approval of the planned labels.
- Phase 8 surfaced the command in help, `which`, README, SKILL, `.printing-press.json`, and `.printing-press-patches/`.
- Verification passed: `go test ./internal/canvaswrite ./internal/cli`, `go test ./...`, `go run ./cmd/affine-pp-cli canvas label-plan --help`, `go run ./cmd/affine-pp-cli which "connector label" --json`, and `canvas apply --dry-run --json` with a `canvas_connector_labels` stdin plan.
- Review plan created from live board connectors: `.planning/milestones/v1.2-connector-label-writing/1co-connector-label-plan.review.json`.
- Human-readable review table created: `.planning/milestones/v1.2-connector-label-writing/1co-connector-label-review.md`.
- Operator approved live apply on 2026-06-20. `canvas-labels-0674943e` applied 23 connector labels to AFFiNE doc `B6pvUw-r5SSfWKam-wncU`.
- Post-apply verification passed: live apply returned `applied: true`, post-integrity returned `ok: true` with `issue_count: 0`, and live search matched all 23 planned labels with 0 mismatches.

## Next Action

Commit or otherwise preserve the v1.2 implementation, planning artifacts, and Showrunner affine-cli skill update.

## Deferred Items

Items acknowledged and deferred at milestone close on 2026-06-19:

| Category | Item | Status |
|---|---|---|
| workflow-proof | Add search, diff, and doc-integrity smoke steps to `workflow_verify.yaml` | deferred |
| live-apply-proof | Assert post-reload fields for every transform operation | deferred |
| gsd-process | Generate phase summaries and Nyquist validation artifacts in future milestones | deferred |
| requirements-traceability | Use formal traceability table in next milestone requirements | deferred |
