<h1 align="center">dg-statuslines</h1>

<p align="center">
  A status line builder for Claude Code.<br>
  Drag widgets into a grid, pick a style, see it live.
</p>

<p align="center"><sub>Go · terminal UI · MIT</sub></p>

<p align="center">
  <a href="#features">Features</a> ·
  <a href="#install">Install</a> ·
  <a href="#use">Use</a> ·
  <a href="#where-it-runs">Where it runs</a> ·
  <a href="ROADMAP.md">Roadmap</a> ·
  <a href="#license">License</a>
</p>

One binary, two commands: `dgsl edit` opens the builder, and `dgsl render` is what
Claude Code runs to draw the status line. No bash script to generate, no `jq`.

## Features

- **Grid layout**: rows of cells, filled by drag and drop. Columns line up like a CSS
  grid, and each cell sits left, center or right in its column.
- **33 widgets**: model (with `✳` and version), directory, git branch, user@host, cost,
  lines added and removed, context, 5h and 7d limits with their reset times, free text,
  and a spacer to leave a real gap. 20 more wait in the **widget catalogue** (`+`): session
  name and time, Claude Code version, effort, fast mode, repository, worktree, pull request,
  tokens, prompt cache, spend limit… Only the widgets you pick show in the list.
- **14 progress bar styles** for the gauges, with caps, width, used or left, and colour
  thresholds.
- **15 themes**: Classic, Catppuccin, Nord, Dracula, Tokyo Night, Gruvbox, Rosé Pine,
  Solarized, One Dark, Monokai, Everforest, Kanagawa, Ayu, GitHub Dark and Mono.
- **Style**: 11 separators, 5 borders around the whole line, blank lines between rows,
  spaced columns, 24 colours plus truecolor themes.
- **7 presets** to start from, and a **random** layout with a history to step back.
- **Live preview** pinned at the top of the builder.
- **Safe install**: only the `statusLine` key of `~/.claude/settings.json` changes, after a
  backup to `settings.json.bak`.

## Install

Linux, macOS and WSL:

```bash
curl -fsSL https://raw.githubusercontent.com/te-o-o-o/dg-statuslines/main/install.sh | sh
```

Windows, in PowerShell:

```powershell
irm https://raw.githubusercontent.com/te-o-o-o/dg-statuslines/main/install.ps1 | iex
```

Both scripts download the binary of the latest release for your system, check it against
the release checksums, and put it in `~/.local/bin` (`%LOCALAPPDATA%\dgsl` on Windows).
Then run `dgsl`, build your line, and install it from the **Install** tab.

To update, run the same command again: the binary is replaced in place, so Claude Code
keeps running it from the same path. The binaries are not signed, so macOS or Windows may
warn the first time.

From source, with Go 1.27 or later:

```bash
git clone https://github.com/te-o-o-o/dg-statuslines && cd dg-statuslines
go install .
```

To uninstall, delete the binary, then restore `~/.claude/settings.json.bak` or remove the
`statusLine` key from `~/.claude/settings.json`.

### macOS

- `~/.local/bin` is not on the `PATH` by default. With zsh, the default shell:

  ```bash
  echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc
  ```

- Installed with the script, the binary is not quarantined. Downloaded from the release
  page in a browser, macOS refuses to open it until you clear the flag:

  ```bash
  xattr -d com.apple.quarantine ~/.local/bin/dgsl
  ```

- Themes use truecolor. If colours look off in Terminal.app, try iTerm2, Ghostty, kitty
  or WezTerm.

## Use

| Key | Action |
|---|---|
| `dgsl` | open the builder (`dgsl help` lists the commands, `dgsl version` prints the version) |
| `1` `2` `3`, `Tab` | Layout, Style and Install tabs |
| drag | place a widget, move or swap cells, drop on the widget list to remove |
| `←↑↓→` | select a cell |
| `[` `]` | previous or next settings section |
| `x` | remove the selected widget |
| `+` | open the widget catalogue: type to filter, `space` to add or remove a widget |
| `r` / `R` | random layout / previous roll |
| `s` | save |
| `q` | quit |

The layout is saved in `~/.claude/dg-statuslines.json`. `dgsl render` reads it, reads the
session JSON Claude Code sends on stdin, and prints the lines. It never fails: bad input or
a missing config gives an empty line, not an error.

## Where it runs

| System | Status |
|---|---|
| Linux, WSL in Windows Terminal | in daily use |
| macOS | built, not tested |
| Windows, outside WSL | built, not tested |

The 5h and 7d limits only exist for Claude Pro and Max accounts, after the first answer
of a session. Without them, those widgets simply stay empty.

## Release

Push a version tag: `git tag v0.1.0 && git push origin v0.1.0`. The release workflow
runs the tests, builds the six binaries (Linux, macOS and Windows, amd64 and arm64) and
publishes them with their checksums.

## License

MIT, see [LICENSE](LICENSE).
