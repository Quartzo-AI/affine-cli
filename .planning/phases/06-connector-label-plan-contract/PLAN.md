# Phase 6: Connector Label Plan Contract

## Goal

Create a deterministic, reviewable connector-label plan contract that can be inspected before any live AFFiNE mutation.

## Context

The user-facing workflow needs a pause between preparing label insertions and applying them. The CLI should emit a plan that is easy for an operator to review and safe for `canvas apply` to consume later.

## Work Items

1. Define `canvas_connector_labels` plan type with plan ID, doc ID, source summary, affected connector IDs, before/after labels, validation warnings, backup target, rollback fields, and proof fields.
2. Add a planner command or transform extension that accepts connector IDs plus label text, and optionally source/target selectors.
3. Support batch input from JSON so operators can prepare many arrow labels at once.
4. Reject ambiguous selectors, missing connectors, duplicate connector updates, unsupported multi-line labels, and invalid UTF-8.
5. Keep planning read-only and compatible with `canvas apply --dry-run`.
6. Add tests for deterministic plan IDs and review JSON shape.

## Verification

- Package tests for `library/affine/internal/canvaswrite` and `library/affine/internal/cli`.
- CLI dry-run emits before/after label values and no live socket push path is reachable.
- Invalid batch input fails before plan output.

## Exit Criteria

Operators can prepare all connector label insertions, review them as structured JSON, and stop before live apply.
