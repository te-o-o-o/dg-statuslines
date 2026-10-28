# Roadmap

Ideas for dg-statuslines that are not built yet, grouped by area. Nothing here is a promise:
it is the list we pick from next. Suggestions are welcome in the issues.

## Widgets

### From the session JSON

Claude Code already sends these fields, so they cost nothing to render.

Already in the catalogue: effort, fast mode, thinking, session name and time, API time,
Claude Code version, output style, vim mode, agent, project, added directories,
repository, worktree, pull request, tokens, context size, over 200k, prompt cache and the
spend limit gauge.

- [ ] **Model**: raw model id.
- [ ] **Session**: short session id.
- [ ] **Context**: cache reads and writes from the last call.
- [ ] **Spend limit**: amount spent against the limit, with its reset time.
- [ ] **Pull request**: a clickable link.
- [ ] **Prompt cache**: time left before it expires, last miss cause.

### Derived from the session JSON

- [ ] **Pace** for the 5h and 7d limits: usage compared with the time elapsed in the window.
- [ ] **Depletion time**: when a limit runs out at the current pace.
- [ ] **Cost per hour**, **tokens per minute**, and the share of time spent on the API.

### Computed locally

- [ ] **Detailed git status**: dirty marker, ahead and behind, staged, modified and
      untracked counts, from a single `git status` call.
- [ ] **More git**: stash count, time since the last commit, current tag.
- [ ] **Clock and date**.
- [ ] **Custom command**: any shell command, with a timeout and a cache.
- [ ] **Environment variable**: show the value of a variable such as `AWS_PROFILE`.

### Heavier, for later

- [ ] **Cost per day or week** across all sessions, read from the transcripts.
- [ ] **MCP servers status**.

## Progress bars

- [ ] Colour gradient along the bar.
- [ ] Pace marker on the 5h and 7d bars, showing the time elapsed in the window.
- [ ] Custom fill and empty characters.
- [ ] Compact styles: a one-character gauge, a pie (`○ ◔ ◑ ◕ ●`), stars.

## Styling

- [ ] Powerline and capsule segments, with a fallback for terminals without a Nerd Font.
- [ ] Bold, dim and italic per widget.
- [ ] Background colour per cell.
- [ ] An icon per widget, and a label that can be renamed or hidden.
- [ ] A title in the top edge of the border, and a border as wide as the terminal.

## Layout

- [ ] Hide a widget when it is empty, zero or below a threshold, separator included.
- [ ] Maximum width per cell, with an ellipsis.
- [ ] Follow the terminal width, and keep room for Claude Code notifications.
- [ ] Group widgets inside brackets.

## Builder

- [ ] Style tab as vertical lists, with a theme preview before applying it.
- [ ] Restore the previous status line from the Install tab.
- [ ] Place a widget from the keyboard.
- [ ] Preview scenarios: context nearly full, a limit close to its end, no git, API key
      account. A slider for the preview width.
- [ ] Undo and redo.
- [ ] Hand pointer over draggable items, in terminals that support it.
- [ ] A more compact layout overall.

## Configuration

- [ ] Export, import and share a layout as a short code.
- [ ] Import an existing ccstatusline or claude-powerline configuration.
- [ ] Set Claude Code's `refreshInterval` from the Install tab.
- [ ] A status line for subagents (`subagentStatusLine`).
- [ ] A per-project configuration.
- [ ] `dgsl doctor`: check the install, the config and the terminal.
- [ ] Honour `NO_COLOR`, and a plain mode for terminals with few colours.

## Nerd ideas

Just ideas, to dig into later. Top three first.

- [ ] **Companion**: a small ASCII creature that lives in the line and reacts to the session:
      asleep when the cache is cold, restless past 80 % context, eating tokens, panicking
      near the 5h limit. Animated with Claude Code's `refreshInterval`.
- [ ] **Sparklines**: `ctx ▁▂▃▅▆█`, `$ ▁▁▂▄▇`, tokens per minute, from a small history kept
      per `session_id` in `~/.cache/dg-statuslines/`.
- [ ] **Pace and depletion**: a `│` marker on the 5h and 7d bars for the time elapsed in the
      window, and "5h empty at 16:40 at this pace".
- [ ] **Template widget**: any field of the session JSON through `text/template`, such as
      `{{.cost.total_cost_usd | printf "%.3f"}}`.
- [ ] **Cache countdown**: "❄ cold in 3m12s" from `prompt_cache.expires_at`.
- [ ] **Clickable links** (OSC 8) on the pull request, the repository and the branch.
- [ ] **Layout codes**: `dgsl export` prints `dgsl:H4sIA…` (gzip + base64), `dgsl import`
      loads one. Share status lines like Minecraft seeds.
- [ ] **Subagent rows** with a sparkline each, from `tokenSamples` in `subagentStatusLine`.
- [ ] **Powerline segments**: coloured blocks chained by `` arrows (rounded, slanted and
      flame variants), with a fallback for terminals without a Nerd Font.
- [ ] **Truecolor gradients** along the bars.
- [ ] **System widgets**: load average, battery, `kubectl` context, `AWS_PROFILE`, the song
      playing (`playerctl`).
- [ ] **Easter egg**: the Konami code in the builder starts a Matrix rain in the preview.

## Beyond the status line

What else Claude Code lets us draw or change, from its docs. Ideas for a companion plugin.

- [ ] **Mod pane**: a sidebar beside the transcript (or a framed region above the prompt),
      with tabs, buttons and fields. A live dashboard: context chart, cost, limits.
- [ ] **Mod band**: the strip above the prompt, redrawn on a timer. Home for the companion.
- [ ] **Redraw Claude Code's own parts** from a mod: the spinner, tool call rows, the hint
      under the prompt, the footer mode labels.
- [ ] **`/dgsl` command** from a mod: open the builder or switch preset without leaving
      the session.
- [ ] **Claude Code colour theme** (`~/.claude/themes/`, shippable in a plugin) matched to
      each dgsl theme.
- [ ] **Spinner verbs and tips** (`spinnerVerbs`, `spinnerTipsOverride`): a themed pack,
      e.g. "Bzzzing…", "Compiling the hive…".
- [ ] **Notification hook**: a sound or desktop notification when Claude finishes or waits.
