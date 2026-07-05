# Verification: Phase 7 Gated Connector Label Live Apply

## Required Checks

- Unit tests for label-only mutation.
- Unit tests for source/target mismatch rejection.
- Unit tests for backup and proof fields.
- Fixture-gated live smoke with explicit operator approval.

## Evidence To Record

- Backup artifact paths.
- Delta artifact paths.
- Pre-integrity result.
- Post-integrity result.
- Reload assertion showing expected label values.

## Evidence Recorded

- `go test ./internal/canvaswrite ./internal/cli` passed.
- `go test ./...` passed.
- Unit coverage verifies label-only JS mutation preserves connector source/target and other fields.
- Validation requires `--live`, `--workspace`, `--doc`, `--backup-dir`, and `--yes` before live write.
- Live AFFiNE smoke intentionally pending. The operator requested review before inserting labels into the board.
