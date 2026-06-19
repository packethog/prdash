# prdash

[![ci](https://github.com/packethog/prdash/actions/workflows/ci.yml/badge.svg)](https://github.com/packethog/prdash/actions/workflows/ci.yml)

A small terminal dashboard for your GitHub pull requests: the ones you
**authored** and the ones **awaiting your review**, across every repo your
active `gh` account can see. Shows per-PR review and CI state, and lets you
merge an approved, green PR from the TUI.

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea); it shells
out to the [`gh`](https://cli.github.com) CLI for all GitHub access, so it reuses
your existing auth.

## Features

- Two buckets — **Authored** and **Awaiting my review** — from a single
  GraphQL query, deduped.
- Per-PR **review** badge (`Approved` / `Changes requested` / `Commented` /
  `Pending review` / `Draft`) and **CI** badge (`✓ / · / ✗ / –`).
- **Merge from the TUI**, hard-gated for safety (see below).
- Graceful network handling: last-known data stays on screen with a
  `Live` / `Stale` / `Offline` indicator and backoff retries.
- Cursor-following scroll, full-width row highlight, and relative times that
  update every second.

## Requirements

- The [`gh`](https://cli.github.com) CLI, authenticated (`gh auth status`),
  with `repo` and `read:org` scope.
- Go 1.24+ to build.

## Install

**Prebuilt binary** (from the [latest release](https://github.com/packethog/prdash/releases/latest)) — pick your platform:

```bash
# macOS arm64 (Apple Silicon); swap for darwin_amd64 / linux_amd64 / linux_arm64
curl -sSL https://github.com/packethog/prdash/releases/latest/download/prdash_darwin_arm64.tar.gz | tar -xz
./prdash_darwin_arm64
```

**With Go:**

```bash
go install github.com/packethog/prdash@latest   # onto your PATH
# or build locally:
go build -o prdash .
```

## Usage

```bash
prdash                 # 45s auto-refresh, 50 PRs/bucket
prdash --interval 30   # refresh every 30s (min 5)
prdash --limit 25      # fetch up to 25 PRs per bucket (min 1)
```

### Keys

| Key | Action |
|-----|--------|
| `↑`/`k`, `↓`/`j` | move cursor |
| `tab` | switch bucket (Authored ↔ Awaiting my review) |
| `r` | refresh now |
| `m` | merge selected (authored only; opens confirm) |
| `c` | close selected (authored only; opens confirm) |
| `o` | open selected PR in browser |
| `v` | review selected with Claude/Codex (awaiting-review bucket, cmux only, when configured) |
| `q` / `ctrl+c` | quit |

In the merge modal: `←`/`→` (or `s`) cycle method, `enter` confirms (only when
approved + CI green), `esc` cancels.

## Behavior notes

- Merge is **hard-gated**: enabled only when the PR is approved
  (`reviewDecision == APPROVED`), CI is passing, it is not a draft, it is
  mergeable (no conflicts / known mergeability), and the connection is `Live`.
  Default method is squash with branch deletion; `gh` re-validates server-side.
- Closing a PR (`c`) is authored-only and asks for confirmation; it keeps the
  branch and is reversible (reopen on GitHub). Like merge, it requires a `Live`
  connection.
- Network drops are handled gracefully: the last-known list stays on screen, the
  footer shows `Live`/`Stale`/`Offline`, and failed fetches retry with backoff.
- Opening a PR (`o`): inside [cmux](https://github.com/manaflow-ai/cmux) it docks
  a browser pane *below* the terminal (`cmux new-pane --direction down`); elsewhere
  it uses `gh pr view --web`.
- Per-workspace PR views are intentionally **not** included — use cmux's native
  `sidebar.showPullRequests` for that. See `docs/superpowers/specs/`.
- The list scrolls to keep the selected PR visible on short terminals; relative
  times update once per second.
- Review badges: `Approved`, `Changes requested`, `Commented` (reviewed with
  feedback, no decision yet), `Pending review`, `Draft`.

### Review launcher (cmux only)

Inside [cmux](https://github.com/manaflow-ai/cmux), pressing `v` on a PR in the
**Awaiting my review** bucket spawns a new agent surface (Claude or Codex) seeded
with a prompt you configure, with the PR's fields substituted in. prdash only
launches the surface — the prompt defines what the agent does (review, post
comments, etc.).

Configure it in `~/.config/prdash/config.toml` (honoring `$XDG_CONFIG_HOME`):

```toml
[review]
provider = "claude"   # "claude" | "codex"
prompt = "Run the consensus-pr-review skill on {{.URL}}. If unavailable, stop and report."
```

Prompt template fields: `{{.URL}}`, `{{.Repo}}`, `{{.Number}}`, `{{.Title}}`,
`{{.Branch}}`. When the config is missing/invalid, or you are not under cmux, the
`v` key is hidden and inert.
