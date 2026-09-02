# Gooo Protected Change-Gate Projector

This repository contains a read-only projector for an append-only Git/GitHub receipt stream. It classifies a substantive implementation, maintenance, or release-plumbing path as `REFUTED`, `UNKNOWN`, or `CLOSED`, with precedence `REFUTED > UNKNOWN > CLOSED`.

The bounded authorization order is:

`AUTHOR_BRANCH -> OPEN_PR -> PR_ACTIONS_GREEN -> MERGE -> MAIN_ACTIONS_GREEN -> POLICY_LOCK -> ANNOTATED_TAG -> DRAFT_RELEASE -> UPLOAD_NEW_ASSETS -> VERIFY_TAG_TARGET_AND_ASSET_DIGEST -> PUBLISH -> IMMUTABLE_AUDIT`

`POLICY_LOCK` is the explicit one-time guard required before release plumbing. Maintenance and release hardening remain substantive changes and must use the pull-request path. A direct-main receipt is retained as `OPERATIONAL_REFUTED`; it is never rewritten.

The projector never commits, merges, tags, creates or publishes releases, replaces assets, mutates policy, or performs destructive operations. It emits only the authorized next operation and the minimal causal frontier. Exact release IDs make interrupted draft-release resumption idempotent. A transient draft-list 404 is preserved as operational provenance and does not override an exact release-ID receipt.

## Contract and fixtures

The executable metacode is [.gooo](/Users/alice/Documents/Codex/2026-09-02/gooo-protected-change-gate-projector/.gooo). It declares exactly 12 cells and 12 activities: four `FOUNDATION`, four `COHERENCE`, and four `REGRESSION`; and four each of `DRIVER`, `OUTCOME`, and `GUARDRAIL`. There is no scalar output.

The deterministic fixture set has exactly 12 vectors: normal implementation PR, normal maintenance PR, direct-main implementation, direct-main release-plumbing, missing PR CI, stale main CI, lightweight tag, tag-target mismatch, publish before policy, publish before asset verification, immutable-asset replacement, and exact-ID interrupted-release resumption.

The fixture file also contains explicit historical event shapes for the observed failure classes. It does not query sibling repositories or depend on live repository state.

## Validation boundary

Go 1.27.x is selected in GitHub Actions. Generate, fix, format, build, vet, test, conformance, replay, and measurement are Actions-only operations; local validation count is `0`. The workflow uses `github.token`, has zero cross-project required gates, and uploads generated replay, measurement, inventory, and checksum evidence as retained Actions artifacts. Root `README.md` is excluded from the Go/Gooo line inventory.

The release workflow preserves failed runs, tags, releases, drafts, and assets. The release path is draft-first: create or resume the draft by exact ID, upload only new asset names, verify the tag target and asset digest, then publish and audit.

## Read-only usage

The CLI accepts a caller-owned output path. With no `--out`, it writes JSON to stdout:

```text
go run ./cmd/projector --events fixtures/cases.json --out /tmp/gooo-replay.json
```

The command is intended to run in the Actions workflow. `--measure` records wall time and RSS only when the host exposes the metric; otherwise it emits null plus an `UNKNOWN` measurement frontier.
