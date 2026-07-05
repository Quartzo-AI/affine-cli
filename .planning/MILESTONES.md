# Milestones

## v1.1 Canvas Power Tools

**Shipped:** 2026-06-19
**Status:** complete with accepted tech debt
**Audit:** `.planning/v1.1-MILESTONE-AUDIT.md`
**Roadmap archive:** `.planning/milestones/v1.1-ROADMAP.md`
**Requirements archive:** `.planning/milestones/v1.1-REQUIREMENTS.md`

### Delivered

AFFiNE CLI now has Canvas Power Tools: block-level search, semantic diff, machine-applyable transform planning, and gated live apply with backups, integrity checks, reload verification, workflow proof, publish validation, and live AFFiNE smoke evidence.

### Key Accomplishments

- Built `canvas search` with reusable selectors for Canvas blocks/cards/connectors.
- Built `canvas diff` with stable semantic categories.
- Built `canvas transform` dry-run plans accepted by `canvas apply --dry-run`.
- Built `canvas apply --live` for transform and layout plans behind explicit safety gates.
- Proved live behavior by writing 20 Live Zap cards and 28 connectors to the approved AFFiNE document.

### Known Deferred Items

See `.planning/v1.1-MILESTONE-AUDIT.md`.

- Durable workflow proof should add search, diff, and doc-integrity smoke steps.
- Live apply should eventually assert post-reload field equality for every operation.
- Phase summaries and Nyquist validation artifacts were not generated for v1.1.

## v1.2 Connector Label Writing

**Status:** planned
**Plan index:** `.planning/milestones/v1.2-connector-label-writing/PLAN-INDEX.md`
**Roadmap:** `.planning/milestones/v1.2-ROADMAP.md`
**Requirements:** `.planning/milestones/v1.2-REQUIREMENTS.md`

### Goal

AFFiNE CLI should safely write text labels onto existing Canvas connector arrows without moving cards, relinking endpoints, or rebuilding the surrounding diagram.

### Required Capability

- Read connector labels from `affine:surface` connector elements.
- Search and diff connectors by label text, source, target, and surface element ID.
- Produce reviewable connector-label plans before any live mutation.
- Apply labels live only behind the existing AFFiNE Canvas gates: workspace, doc, backup directory, explicit confirmation, pre/post integrity, reload verification, and proof artifacts.
- Preserve connector geometry, endpoints, stroke, arrowheads, seed, style, and current card positions.

### Product Rule

Connector label insertion must support a review-first workflow. Operators prepare label text, inspect the planned before/after values, and only then run a separate live apply.
