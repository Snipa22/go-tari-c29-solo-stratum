---
description: go-tari-* ecosystem repo — Tari network Go tooling
---

# AGENTS.md

Instructions for AI coding agents (OpenCode, Claude Code, or any `agents.md`-compatible tool) working in this repository. Read this before making changes.

## Project

- **What this repo is:** Solo stratum mining server for Tari's C29 (Cuckaroo29) proof-of-work algorithm.
- **Module path:** `github.com/snipa22/go-tari-c29-solo-stratum`
- **Depends on:** `go-tari-grpc-lib` (GRPC wrapper), `go-tari-lib` (helpers, if used), `core-go-lib` (shared utilities)

## Commands

- **Build:** `go build ./...`
- **Test:** `go test ./...`
- **Vet:** `go vet ./...`
- **Format:** `gofmt -l .` (should return nothing; `gofmt -w .` to fix)
- **Tidy:** `go mod tidy`

Run build + vet + gofmt + test before considering any change complete. CI will re-check all four; catch failures locally first.

## Conventions

- **Conventional Commits** required — commit type (`feat`/`fix`/`chore`/etc.) drives automated SemVer via release-please. Don't guess the type; pick the one that matches the actual change.
- **Rebase, never merge.** No merge commits in PR branches. Rebase onto `main` before pushing updates.
- **No direct commits/pushes to `main`.** Always via PR.
- Follow existing package structure and naming — don't introduce a new pattern without checking how sibling `go-tari-*` repos do it first (they should be consistent; if they're not, that's a bug to flag, not a license to add a third way).
- Pin dependency versions explicitly in `go.mod` — this ecosystem has a known history of version skew across repos on `go-tari-grpc-lib`; don't make it worse.

## Don't

- Don't push directly to `main` or force-push shared branches.
- Don't add merge commits — rebase instead.
- Don't touch generated/vendored code (anything under a `_generated`, `tari_generated`, `tari_protos`, or similar directory) by hand — regenerate it from source instead.
- Don't silently change the licensing header or LICENSE file — that's a human decision, flag it instead.
- Don't skip tests because "there weren't any before" — add coverage for what you touch.

## Disclosure

If you (the agent) are making a substantial autonomous contribution, make sure the human operator adds a disclosure note to the PR per `CONTRIBUTING.md`. Don't assume this happens automatically — mention it if it's about to be skipped.
