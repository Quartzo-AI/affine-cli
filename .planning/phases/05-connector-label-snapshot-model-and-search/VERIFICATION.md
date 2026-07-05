# Verification: Phase 5 Connector Label Snapshot Model And Search

## Required Checks

- Unit tests for label extraction from `affine:surface.prop:elements`.
- Unit tests for unlabeled connectors.
- Unit tests for `connector_label_changed` diff category.
- CLI help remains valid for `canvas search`, `canvas diff`, and `canvas model`.

## Evidence To Record

- Test command and result.
- Example JSON showing connector ID, source, target, label text, and label style.
- Any uncertainty about AFFiNE's canonical label field.

## Evidence Recorded

- `go test ./internal/canvaswrite ./internal/cli` passed.
- `go test ./...` passed.
- `SearchEntity` now exposes `connector_label` and `label_style` for connector elements.
- Connector label extraction reads `text` first, with compatibility fallbacks for `label` and `labelText`; uncertainty remains that AFFiNE may formalize a different canonical field later.
- Semantic diffs now report connector label changes as `connector_label_changed`.
