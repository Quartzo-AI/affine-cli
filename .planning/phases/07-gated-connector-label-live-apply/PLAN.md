# Phase 7: Gated Connector Label Live Apply

## Goal

Apply reviewed connector label plans to live AFFiNE documents while preserving every non-label connector field.

## Context

v1.1 already has live Canvas apply gates for transform and layout plans. Connector label writing should reuse that mutation posture instead of adding a separate unsafe writer.

## Work Items

1. Add live apply support for `canvas_connector_labels` plans.
2. Load the AFFiNE document, run pre-integrity, and write backup-before artifacts.
3. Locate existing connector elements by surface element ID and verify expected source/target before mutation.
4. Set only approved label fields and preserve source, target, stroke, endpoint styles, seed, label style, geometry, and card positions unless the plan explicitly changes label style.
5. Encode and push a Y.js delta only after local integrity passes.
6. Reload the document and assert every planned label value is present.
7. Return proof fields that make review and rollback possible.

## Verification

- `go test ./...` from `library/affine`.
- Apply-script tests prove only label fields change.
- Live fixture smoke only after explicit approval.
- Pre/post integrity and reload verification evidence are recorded.

## Exit Criteria

Connector labels can be applied live with the same safety profile as v1.1 Canvas apply, and verification proves the operation is label-only.
