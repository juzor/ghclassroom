# ghclassroom

Terminal UI for browsing GitHub Classroom — classrooms, assignments, student submissions, and repo activity.

## Requirements

- Go 1.22 or later
<!-- - macOS or Linux -->

## Install

```
go install ghclassroom@latest
```

Or build from source:

```
git clone https://github.com/your-org/ghclassroom
cd ghclassroom
go install .
```
Or build a binary using make
```
git clone https://github.com/your-org/ghclassroom
cd ghclassroom 
make build-bin
```
Or build an executable (Windows) using make
```
git clone https://github.com/your-org/ghclassroom
cd ghclassroom 
make build-exe
```

## First run

On first launch you will be prompted for a GitHub personal access token:

```
GitHub personal access token required.
Required scopes: repo, read:org
Generate one at: https://github.com/settings/tokens

Paste token:
```

The token is stored in `~/.config/ghclassroom/config.json` and reused on subsequent runs. To reset it:

```
ghclassroom --reconfigure
```

Required token scopes:

| Scope | Why |
|-------|-----|
| `repo` | Read student repository contents and commits |
| `read:org` | List classrooms and assignments via the Classroom API |

## Key bindings

| Key | Action |
|-----|--------|
| `↑` `↓` or `j` `k` | Move selection |
| `→` or `Enter` | Drill into selected item |
| `←` or `Esc` | Go back |
| `/` | Filter assignments (fuzzy search) |
| `o` | Open selected URL in browser |
| `c` | Copy URL to clipboard |
| `r` | Refresh current panel (clears cache) |
| `q` or `Ctrl+C` | Quit |

## Limitations

<!-- - **macOS and Linux only.** Windows is not supported (`xdg-open` / `open` are used for browser launch). -->
- **No auto-refresh.** Data is cached for the session; press `r` to reload a panel.
- **Read-only.** ghclassroom never writes to GitHub — no assignment creation, no grading, no repo modification.
- **API rate limits.** The GitHub API allows 5 000 requests per hour for authenticated users. Heavy use across many classrooms can approach this limit; a countdown is shown if it is reached.
