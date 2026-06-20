# prdash

[![ci](https://github.com/packethog/prdash/actions/workflows/ci.yml/badge.svg)](https://github.com/packethog/prdash/actions/workflows/ci.yml)

A terminal dashboard for GitHub pull requests and CI workflow runs. Shows the
PRs you **authored** and the ones **awaiting your review** across every repo
your active `gh` account can see, alongside a **CI Workflows** section that
tracks the last N runs of any configured GitHub Actions workflows. Lets you
merge an approved PR, rerun failed CI jobs, and dispatch an AI debug session
— all without leaving the TUI.

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea); it shells
out to the [`gh`](https://cli.github.com) CLI for all GitHub access, so it
reuses your existing auth.

## Features

- Two PR buckets — **Authored** and **Awaiting my review** — from a single
  GraphQL query, deduped.
- Per-PR **review** badge (`Approved` / `Changes requested` / `Commented` /
  `Pending review` / `Draft`) and **CI** badge (`✓ / · / ✗ / –`).
- **CI Workflows** section: last-N runs per configured workflow, sparkline
  status, expandable run list, details modal with optional failure-analysis
  summary fetched from an uploaded artifact.
- **Merge from the TUI**, hard-gated for safety (see below).
- **Rerun failed jobs** (`R`) from the TUI.
- **Debug dispatch** (`d`) — spawns the configured AI provider (Claude or
  Codex) in a cmux pane with a rendered prompt, exactly like the PR review
  launcher.
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
| `tab` | rotate section (Authored → Awaiting my review → CI Workflows) |
| `r` | refresh now |
| `m` | merge selected (authored only; opens confirm) |
| `c` | close selected (authored only; opens confirm) |
| `o` | open selected PR or CI run in browser |
| `v` | review selected with Claude/Codex (awaiting-review bucket, cmux only, when configured) |
| `↵` | CI: expand/collapse a workflow header; open run details modal |
| `d` | CI: debug dispatch (failed run selected or in details modal; cmux only, when configured) |
| `R` | CI: rerun failed jobs (failed run selected or in details modal; opens confirm) |
| `q` / `ctrl+c` | quit |

In the merge modal: `←`/`→` (or `s`) cycle method, `enter` confirms (only when
approved + CI green), `esc` cancels.

In the details modal: `↑`/`↓` scroll, `↵`/`esc` close, `o` open run page,
`d` debug (failed runs only), `R` rerun (failed runs only).

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
- Opening a PR or CI run (`o`): inside [cmux](https://github.com/manaflow-ai/cmux)
  it docks a browser pane *below* the terminal (`cmux new-pane --direction down`);
  elsewhere it uses `gh pr view --web` (PR) or opens the run URL directly.
- Per-workspace PR views are intentionally **not** included — use cmux's native
  `sidebar.showPullRequests` for that. See `docs/superpowers/specs/`.
- The list scrolls to keep the selected PR visible on short terminals; relative
  times update once per second.
- Review badges: `Approved`, `Changes requested`, `Commented` (reviewed with
  feedback, no decision yet), `Pending review`, `Draft`.

### Review launcher (cmux only)

Inside [cmux](https://github.com/manaflow-ai/cmux), pressing `v` on a PR in the
**Awaiting my review** bucket opens a new terminal pane below
(`cmux new-pane --type terminal --direction down`) and runs
`<provider> <args…> '<prompt>'` in it — i.e. it launches your configured agent
CLI (Claude or Codex) with the prompt, the PR's fields substituted in. prdash
only launches the pane and the command — the prompt defines what the agent does
(review, post comments, etc.).

### CI Workflows section

The **CI Workflows** section is the third section in the TUI (press `tab` twice
from Authored, or once from Awaiting my review). It tracks the last N runs of
each configured workflow and shows them as a sparkline of status glyphs.

**Status glyphs:**

| Glyph | Meaning |
|-------|---------|
| `✓` | success (green) |
| `✗` | failure (red) |
| `⊘` | cancelled (dim) |
| `◐` | queued or in progress (yellow) |
| `–` | unknown / no runs |

**Navigation:**

- `↑`/`↓` move the cursor over workflow headers and (when expanded) run rows.
- `↵` on a **workflow header** — expand or collapse it. Collapsed rows show a
  sparkline of the last N runs. Expanded rows show each run individually with
  its number, status label, and age.
- `↵` on an **expanded run row** — open the run details modal (see below).
- `o` — open the selected run's GitHub page in a browser pane.
- `d` — debug dispatch: spawn the configured provider in a cmux pane with a
  rendered debug prompt. Only active for **failed runs**, under cmux, when
  `ci.provider` and `ci.prompt` are configured.
- `R` — rerun failed jobs: opens a confirmation prompt, then runs
  `gh run rerun --failed`. Only active for **failed runs**.

**Details modal** (`↵` on a run row):

The modal shows structured run metadata (status, branch, timing, jobs with the
failed step). When the workflow is configured with `summaryArtifact`, prdash
also fetches the newest matching artifact and renders `summaryFile` (default
`analysis.txt`) as an in-TUI failure analysis summary. When no artifact is
found, the modal shows a hint to press `o` to open the run page instead.

Keys inside the modal: `↑`/`↓` scroll · `↵`/`esc` close · `o` open run page ·
`d` debug (failed runs) · `R` rerun (failed runs).

**In-TUI analysis summary** requires your workflow to upload the analysis file
as an artifact. For example, add an `actions/upload-artifact` step:

```yaml
- uses: actions/upload-artifact@v4
  with:
    name: qa-analysis-${{ github.run_id }}
    path: analysis.txt
```

Then set `summaryArtifact: qa-analysis-*` (and optionally `summaryFile`) in
your `config.yaml`. Without this step in the workflow, the details modal shows
run metadata and an "open the run page" hint — prdash degrades gracefully.

## Configuration

Config lives at `~/.config/prdash/config.yaml` (respecting `$XDG_CONFIG_HOME`).
Both the review launcher and CI workflows are configured here. A missing file
means both features are disabled and prdash runs as a read-only dashboard. A
bad `review` block disables only the review launcher; a bad `ci` block disables
only CI — prdash always starts.

```yaml
review:
  provider: claude                      # "claude" | "codex"
  args: ["--permission-mode", "auto"]  # optional flags before the prompt
  prompt: "Run the consensus-pr-review skill on {{.URL}}."

ci:
  limit: 5                 # default last-N runs per workflow (1–20, default 5)
  provider: claude         # debug-dispatch provider: "claude" or "codex"
  args: ["--permission-mode", "auto"]   # optional flags before the prompt
  prompt: |
    Debug failed CI run {{.URL}} ({{.Workflow}} on {{.Branch}}, run {{.RunID}}).
    Download artifacts with `gh run download {{.RunID}}`; treat any existing
    analysis.txt as a hint and verify it against the logs and code.
  workflows:
    - repo: malbeclabs/infra
      workflow: qa.mainnet-beta.yml   # workflow file name (used by `gh run list --workflow`)
      name: QA mainnet-beta           # optional display label (defaults to workflow file name)
      branch: main                    # optional branch filter
      summaryArtifact: qa-analysis-*  # optional artifact name/glob holding the analysis
      summaryFile: analysis.txt       # optional file within the artifact (default: analysis.txt)
    - repo: malbeclabs/infra
      workflow: qa.testnet.yml
      name: QA testnet
      limit: 10                       # optional per-workflow override of ci.limit
```

### `review` fields

- `provider` — `"claude"` or `"codex"`.
- `args` — optional flags passed before the prompt. For Claude,
  `["--permission-mode", "auto"]` starts it in auto-approval mode.
- `prompt` — a Go `text/template`. Available fields: `{{.URL}}`, `{{.Repo}}`,
  `{{.Number}}`, `{{.Title}}`, `{{.Branch}}`. Each part is shell-quoted.

### `ci` fields

- `limit` — default number of runs to fetch per workflow (clamped to 1–20;
  defaults to 5 when omitted or zero).
- `provider` — `"claude"` or `"codex"`. Required for debug dispatch (`d`).
- `args` — optional flags passed to the provider before the prompt.
- `prompt` — a Go `text/template` for the debug dispatch prompt. Required when
  `provider` is set. Available fields:

  | Field | Example |
  |-------|---------|
  | `{{.URL}}` | run HTML URL |
  | `{{.Repo}}` | `malbeclabs/infra` |
  | `{{.Workflow}}` | display name (e.g. `QA mainnet-beta`) |
  | `{{.Branch}}` | `main` |
  | `{{.RunID}}` | numeric run id (for `gh run download`, etc.) |
  | `{{.RunNumber}}` | GitHub run number |
  | `{{.Conclusion}}` | `failure`, `cancelled`, … |

- `workflows` — list of workflows to track. The `ci` section (and all CI keys)
  is hidden when the list is empty or `ci` is absent entirely.

### Per-workflow fields

Each entry under `ci.workflows` accepts:

| Field | Required | Description |
|-------|----------|-------------|
| `repo` | yes | `owner/name` of the GitHub repository |
| `workflow` | yes | workflow file name, as passed to `gh run list --workflow` |
| `name` | no | display label in the TUI (defaults to `workflow`) |
| `branch` | no | filter runs to this branch |
| `limit` | no | override `ci.limit` for this workflow only |
| `summaryArtifact` | no | artifact name or glob (e.g. `qa-analysis-*`) to fetch the analysis from |
| `summaryFile` | no | file to read inside the artifact (defaults to `analysis.txt`) |
