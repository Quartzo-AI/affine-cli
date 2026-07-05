# Plan Index: v1.2 Connector Label Writing

## Scope

This milestone adds review-first label writing for existing AFFiNE Canvas connector arrows.

## Requirement Coverage

| Requirement | Phase |
| --- | --- |
| LABEL-01 Connector Label Search And Model Output | Phase 5 |
| LABEL-02 Connector Label Diff Classification | Phase 5 |
| LABEL-03 Reviewable Connector Label Plan | Phase 6 |
| LABEL-04 Label-Only Live Mutation | Phase 7 |
| LABEL-05 Existing Canvas Live Apply Gates | Phase 7 |
| LABEL-06 Docs, Help, MCP, Patch Records | Phase 8 |

## Subagent Operating Plan

Use at most two active subagents at any time.

For each phase:

1. Main agent opens the phase and confirms write scope.
2. One implementation lane makes scoped changes.
3. One verification or integration-review lane checks behavior and contracts.
4. Main agent runs final checks, updates `.planning/STATE.md`, and decides whether to advance.

## Phase Plans

- [Phase 5: Connector Label Snapshot Model And Search](../../phases/05-connector-label-snapshot-model-and-search/PLAN.md)
- [Phase 6: Connector Label Plan Contract](../../phases/06-connector-label-plan-contract/PLAN.md)
- [Phase 7: Gated Connector Label Live Apply](../../phases/07-gated-connector-label-live-apply/PLAN.md)
- [Phase 8: Workflow Proof, Docs, And Publish Durability](../../phases/08-workflow-proof-docs-and-publish-durability/PLAN.md)
