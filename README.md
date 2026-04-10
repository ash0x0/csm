# csm — Claude Session Manager

[![CI](https://github.com/ash0x0/csm/actions/workflows/ci.yml/badge.svg)](https://github.com/ash0x0/csm/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/ash0x0/csm)](https://goreportcard.com/report/github.com/ash0x0/csm)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)

Fast CLI tool for managing [Claude Code](https://docs.anthropic.com/en/docs/claude-code) sessions. List, search, merge, diff, move, and delete sessions from the terminal.

## Features

- **Fast listing** — scans all sessions in ~9ms (cached), grouped by project
- **Interactive TUI** — fzf-based interface with preview, multi-select, and collapsible project groups
- **Full merge** — combine multiple sessions into one with complete conversation history preserved
- **Session management** — move between projects, delete with safety checks
- **Search** — find sessions by content, title, or prompt with optional date filtering
- **Timeline** — chronological view of a session's conversation turns with token counts
- **Tasks** — show the task tree for a session with completion status
- **Plans** — list plan files correlated with the sessions that generated them
- **Activity** — GitHub-style contribution heatmap of your Claude Code usage
- **Index repair** — rebuild stale `sessions-index.json` files (fixes `/resume` picker)
- **Storage stats** — disk usage breakdown by project and artifact type (sorted by size)

## Installation

### Quick install (macOS / Linux)

```bash
curl -fsSL https://raw.githubusercontent.com/ash0x0/csm/main/install.sh | sh
```

Or set a custom install directory:

```bash
CSM_INSTALL_DIR=~/.local/bin curl -fsSL https://raw.githubusercontent.com/ash0x0/csm/main/install.sh | sh
```

### Go install

```bash
go install github.com/ash0x0/csm@latest
```

### From release binaries

Download pre-built binaries from [GitHub Releases](https://github.com/ash0x0/csm/releases).

### Build from source

```bash
git clone https://github.com/ash0x0/csm.git
cd csm
make install
```

## Requirements

- [fzf](https://github.com/junegunn/fzf) — required for the interactive TUI (`csm` with no args)

## Quick Start

```bash
# Interactive TUI — browse, merge, diff, delete, move sessions
csm

# List all sessions grouped by project
csm list

# Show session details and prompt history
csm show moonlight     # search by title
csm show 063fad40      # or by ID prefix

# Merge sessions (preserves full conversation history)
csm merge <id1> <id2> <id3>

# Fix the built-in /resume picker
csm reindex
```

## Commands

| Command | Description |
|---------|-------------|
| `csm` | Interactive TUI (default when no args) |
| `csm list` | List sessions grouped by project |
| `csm show <id>` | Show session details, prompt history, files touched, and tasks |
| `csm merge [ids...]` | Merge sessions (git-style dedup of shared history) |
| `csm diff <a> <b>` | Compare two sessions (identical, superset, diverged, unrelated) |
| `csm search <query>` | Search sessions by title or prompt content |
| `csm timeline <id>` | Show chronological conversation timeline with token counts |
| `csm tasks <id>` | Show task tree with completion status |
| `csm plans [slug]` | List plan files correlated with sessions |
| `csm activity` | GitHub-style heatmap of daily Claude Code usage |
| `csm rm [ids...]` | Delete sessions and artifacts |
| `csm move <id> [project]` | Move a session to another project |
| `csm reindex` | Rebuild session indexes |
| `csm stats` | Show storage breakdown sorted by size |
| `csm version` | Print version |

### List flags

| Flag | Description |
|------|-------------|
| `--project`, `-p` | Filter by project path substring |
| `--branch`, `-b` | Filter by git branch |
| `--since`, `-s` | Show sessions modified within duration (e.g. `7d`, `30d`) |
| `--min-messages`, `-m` | Minimum message count |
| `--stale` | Show stale sessions (<3 msgs AND older than 14d) |
| `--all` | Include observer sessions |
| `--fzf` | Compact output for piping to fzf |
| `--json` | JSON output |
| `--sort` | Sort by: `modified` (default), `created`, `messages`, `size` |
| `--orphaned`, `-O` | List projects whose paths no longer exist |

### Show flags

| Flag | Description |
|------|-------------|
| `--files` | Show files modified during the session |
| `--max-prompts` | Number of recent prompts to display (default: 5) |
| `--json` | JSON output |

### Search flags

| Flag | Description |
|------|-------------|
| `--deep` | Also search all user prompts (slower) |
| `--project`, `-p` | Filter by project path substring |
| `--since` | Only show sessions modified after this date (YYYY-MM-DD) |
| `--json` | JSON output |

### Delete flags

| Flag | Description |
|------|-------------|
| `--older-than` | Delete sessions older than duration (e.g. `30d`) |
| `--stale` | Delete stale sessions (<3 msgs AND older than 14d) |
| `--dry-run`, `-n` | Preview what would be deleted |
| `--force`, `-f` | Skip confirmation |
| `--orphaned` | Clean up orphaned artifacts with no matching session |

## Interactive TUI

Run `csm` with no arguments to open the interactive interface:

| Key | Action |
|-----|--------|
| `SPACE` | Select session / fold-unfold project group |
| `ENTER` | Merge selected sessions (2+) |
| `ctrl-d` | Delete highlighted session |
| `ctrl-o` | Move highlighted session to another project |
| `ctrl-f` | Diff two selected sessions |
| `ctrl-t` | Show task tree for highlighted session |
| `ctrl-l` | Show conversation timeline for highlighted session |
| `ctrl-s` | Search sessions (uses current query as search term) |
| `ctrl-p` | Show plans correlated with sessions |
| `ctrl-a` | Show activity heatmap |
| `ESC` | Quit |

All sub-commands return to the TUI after completing. The right pane shows a preview of the highlighted session's details, files touched, and task summary.

## How It Works

Claude Code stores sessions as JSONL files in `~/.claude/projects/`. Each file contains events (user messages, assistant responses, tool calls, system events) linked via `uuid`/`parentUuid` chains.

`csm` scans these files directly with parallel goroutines and caches metadata for sub-10ms subsequent lookups. Session merging preserves the complete event chain by rewriting `sessionId` fields and linking the `parentUuid` chain across sessions.

### Data locations

| Path | Contents |
|------|----------|
| `~/.claude/projects/<project>/*.jsonl` | Session conversation logs |
| `~/.claude/projects/<project>/sessions-index.json` | Session catalog (used by `/resume`) |
| `~/.claude/sessions/*.json` | Active session PID files |
| `~/.claude/tasks/<session-id>/` | Task tracking |
| `~/.claude/session-env/<session-id>/` | Session environment |

## Global flags

| Flag | Description |
|------|-------------|
| `--claude-dir` | Path to Claude Code data directory (default: `~/.claude`) |

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[GPL-3.0](LICENSE)
