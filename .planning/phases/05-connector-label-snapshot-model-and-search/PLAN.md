# Phase 5: Connector Label Snapshot Model And Search

## Goal

Expose connector label text and label style in the read-only Canvas model so later phases can plan and verify label updates without guessing AFFiNE internals.

## Context

Current connector extraction reads surface connector IDs, source IDs, target IDs, and approximate bounds. Connector creation already writes a `labelStyle` field, but search does not expose label content or style. This phase establishes the read contract first.

## Work Items

1. Inspect AFFiNE connector element shape in `affine:surface.prop:elements` and identify the exact label text field used by rendered connector labels.
2. Extend `SearchEntity` for connector label text and relevant label style fields.
3. Extend surface connector extraction to preserve label fields without fabricating empty labels.
4. Extend `canvas model` normalization for connector labels when inspect JSON includes them.
5. Extend semantic diff with a connector-label-specific category that is distinct from endpoint relinks.
6. Add unit tests for labeled connectors, unlabeled connectors, and endpoint relinks.

## Verification

- Package tests for `library/affine/internal/canvaswrite` and `library/affine/internal/cli`.
- Fixture proves connector label text appears in `canvas search` output.
- Diff test proves label changes are not reported as relinks.

## Exit Criteria

Read-only connector label state is available to the CLI and tests prove it can distinguish label changes from structural connector changes.
