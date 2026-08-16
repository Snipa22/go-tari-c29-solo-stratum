# Contributing

This project is part of the `go-tari-*` ecosystem maintained by [Snipa22](https://github.com/Snipa22), providing Go tooling for the [Tari](https://github.com/tari-project/tari) network.

## Workflow

- **Conventional Commits** — all commits must follow [Conventional Commits](https://www.conventionalcommits.org/) format (`feat:`, `fix:`, `docs:`, `chore:`, etc.). Versioning is automated from these via [release-please](https://github.com/googleapis/release-please) — commit type/scope directly determines the next SemVer bump, so get it right.
- **No direct pushes to `main`.** All changes land via pull request.
- **Rebase only — no merge commits, no squash-and-merge.** Keep your branch rebased on `main` before opening/updating a PR; a linear history is required and enforced by branch protection. If your PR shows a merge commit or unrelated history, rebase it (`git rebase main`) rather than merging.
- **1 approving review required** before merge. Repo maintainer can bypass for solo/trusted work; external contributions get the full review bar.
- **CI must be green** before merge (build, vet/lint, tests as configured per-repo — see that repo's `AGENTS.md` for exact commands).

## AI-assisted contributions

This project is developed with significant AI agent assistance (OpenCode, Claude, etc.) as a matter of course — that's normal here, not exceptional. Disclosure exists so reviewers know what to expect from a diff, not to gatekeep AI use.

- **Disclosure:** if a commit or PR was substantially generated or drafted by an AI agent, say so. Practically: add a line to the PR description (`Generated-with: <tool/model>` or equivalent free text) — no special tooling required, just don't hide it.
- **External contributors:** AI-assisted PRs are welcome under the same disclosure rule above. You're still responsible for the correctness of what you submit — "the AI wrote it" is not a defense for an untested or unreviewed change. PRs touching payout, financial, or consensus-adjacent logic get held to a stricter review bar regardless of authorship (see the repo's own README/AGENTS.md for anything repo-specific).
- Repo-specific agent instructions live in `AGENTS.md` at repo root — read it before making agent-driven changes, and keep it updated if your workflow needs something it doesn't cover.

## Testing

New code needs test coverage. If you're touching a repo that currently has none (much of this ecosystem, as of 2026-08 — being actively remedied), adding tests for the code you touch is expected, not optional.

## License

See `LICENSE` at repo root. Repos generating from or deriving Tari core protobuf/source inherit Tari core's license (BSD-3-Clause); everything else in this ecosystem uses a permissive license (MIT unless noted otherwise). Don't assume — check the actual `LICENSE` file for this repo.
