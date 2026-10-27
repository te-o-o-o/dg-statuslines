<h1 align="center">slb</h1>

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
  <a href="#license">License</a>
</p>

One binary, two commands: `slb edit` opens the builder, and `slb render` is what
Claude Code runs to draw the status line. No bash script to generate, no `jq`.

## Features

- **Grid layout**: rows of cells, filled by drag and drop. Columns line up like a CSS
  grid, and each cell sits left, center or right in its column.
- **13 widgets**: model (with `✳` and version), directory, git branch, user@host, cost,
  lines added and removed, context, 5h and 7d limits with their reset times, free text,
  and a spacer to leave a real gap.
- **14 progress bar styles** for the gauges, with caps, width, used or left, and colour
  thresholds.
- **15 themes**: Classic, Catppuccin, Nord, Dracula, Tokyo Night, Gruvbox, Rosé Pine,
  Solarized, One Dark, Monokai, Everforest, Kanagawa, Ayu, GitHub Dark and Mono.
- **Style**: 11 separators, 5 borders around the whole line, blank lines between rows,
  spaced columns, 24 colours plus truecolor themes.
- **6 presets** to start from, and a **random** layout with a history to step back.
- **Live preview** pinned at the top of the builder.
- **Safe install**: only the `statusLine` key of `~/.claude/settings.json` changes, after a
  backup to `settings.json.bak`.

## Install

Go 1.27 or later is all it takes:

```bash
git clone https://github.com/te-o-o-o/dg-statuslines && cd dg-statuslines
go install .
```

This puts `slb` in `$(go env GOPATH)/bin`, usually `~/go/bin`; add it to your `PATH` if it
is not there yet. Then run `slb`, build your line, and install it from the **Install** tab.

Claude Code runs the binary from the path the Install tab writes. After an update, run
`go install .` again: the path stays the same, so nothing needs reinstalling.

## Use

| Key | Action |
|---|---|
| `slb` | open the builder (`slb help` lists the commands) |
| `1` `2` `3`, `Tab` | Layout, Style and Install tabs |
| drag | place a widget, move or swap cells, drop on the widget list to remove |
| `←↑↓→` | select a cell |
| `[` `]` | previous or next settings section |
| `x` | remove the selected widget |
| `r` / `R` | random layout / previous roll |
| `s` | save |
| `q` | quit |

The layout is saved in `~/.claude/statusline-builder.json`. `slb render` reads it, reads the
session JSON Claude Code sends on stdin, and prints the lines. It never fails: bad input or
a missing config gives an empty line, not an error.

## Where it runs

| System | Status |
|---|---|
| Linux, WSL in Windows Terminal | in daily use |
| macOS | not tested |
| Windows, outside WSL | not tested |

The 5h and 7d limits only exist for Claude Pro and Max accounts, after the first answer
of a session. Without them, those widgets simply stay empty.

## License

MIT, see [LICENSE](LICENSE).
