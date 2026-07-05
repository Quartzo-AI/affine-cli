# Phase 8: Workflow Proof, Docs, And Publish Durability

## Goal

Make connector label writing durable in the AFFiNE CLI surface, docs, tests, workflow proof, and Printing Press patch records.

## Context

The feature touches a generated printed CLI under `library/affine`. Hand-authored changes must leave patch records and validation evidence so they survive the Printing Press workflow.

## Work Items

1. Update command help and examples for review-first connector label planning and apply.
2. Ensure MCP annotations reflect read-only planning versus mutating apply.
3. Add or update workflow proof for search, plan, dry-run apply, and fixture-gated live apply.
4. Add Printing Press patch record documenting connector label writing.
5. Update AFFiNE CLI skill/readme surfaces if the command surface changes.
6. Run full validation required for printed CLI changes.

## Verification

- `go test ./...` from `library/affine`.
- Relevant CLI help commands.
- Workflow proof check.
- Publish validation or documented blocker if external publish gates are unavailable.

## Exit Criteria

The connector label writing capability is documented, verifiable, and durable under the Printing Press patch workflow.
