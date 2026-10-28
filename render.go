package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
)

type Cell struct {
	Widget  string `json:"widget,omitempty"`
	Color   string `json:"color,omitempty"` // fixed color, or threshold scheme for gauges
	Text    string `json:"text,omitempty"`
	Bar     string `json:"bar,omitempty"`   // gauge bar style ("" = blocks)
	Mode    string `json:"mode,omitempty"`  // gauges: "" = used, "left" = remaining
	Width   int    `json:"width,omitempty"` // gauges: 0 = 10
	NoLabel bool   `json:"no_label,omitempty"`
	Full    bool   `json:"full,omitempty"`    // directory: full path with ~
	Caps    string `json:"caps,omitempty"`    // gauges: caps ("" = [ ] for ASCII, none otherwise)
	Icon    bool   `json:"icon,omitempty"`    // model: ✳ prefix like the old statusline-command.sh
	Version bool   `json:"version,omitempty"` // model: append the version taken from model.id
	Align   string `json:"align,omitempty"`   // position in its column: "" (left), "center", "right"
	Sep     string `json:"sep,omitempty"`     // separator after this cell ("" = the global one)
}

type Config struct {
	Rows        [][]Cell `json:"rows"`
	Gaps        []bool   `json:"gaps,omitempty"`         // Gaps[i]: blank line after row i
	Free        []bool   `json:"free,omitempty"`         // Free[i]: row i is off the grid (its own widths)
	AirCols     bool     `json:"air_cols,omitempty"`     // more space between cells
	Sep         string   `json:"sep,omitempty"`          // separator between cells ("" = │, "space" = none)
	Border      string   `json:"border,omitempty"`       // border around the whole status line ("" = none)
	BorderColor string   `json:"border_color,omitempty"` // "" = dim gray
	Theme       string   `json:"theme,omitempty"`        // last applied theme, also used for widgets added later
	UI          string   `json:"ui,omitempty"`           // builder appearance ("" = auto); no effect on the render
	Palette     []string `json:"palette,omitempty"`      // widgets listed in the builder (nil = the non-extra ones); no effect on the render
}

// widgetInfo describes a widget for the builder. Group orders the widget list and
// the catalogue; Extra widgets stay in the catalogue until the user adds them.
type widgetInfo struct {
	ID, Label, Group, Desc string
	Extra                  bool
}

// Every widget, in list order. Groups appear in the order of their first widget.
var widgets = []widgetInfo{
	{"model", "Model", "Session", "model name, with ✳ and version", false},
	{"dir", "Directory", "Session", "current directory, short or full", false},
	{"git", "Git branch", "Session", "current branch", false},
	{"userhost", "user@host", "Session", "user and machine", false},
	{"session", "Session name", "Session", "set with --name or /rename, or the AI title", true},
	{"duration", "Session time", "Session", "time the session has been running", true},
	{"apitime", "API time", "Session", "time spent waiting for the API", true},
	{"version", "Claude version", "Session", "Claude Code version", true},
	{"agent", "Agent", "Session", "agent name, with --agent", true},
	{"style", "Output style", "Session", "hidden while it is the default one", true},
	{"vim", "Vim mode", "Session", "NORMAL, INSERT… when vim mode is on", true},
	{"effort", "Effort", "Model", "reasoning effort, low to max", true},
	{"fast", "Fast mode", "Model", "shown while fast mode is on", true},
	{"thinking", "Thinking", "Model", "shown while extended thinking is on", true},
	{"project", "Project", "Workspace", "directory Claude Code was launched in", true},
	{"repo", "Repository", "Workspace", "owner/name of the origin remote", true},
	{"worktree", "Worktree", "Workspace", "linked git worktree name", true},
	{"added", "Added dirs", "Workspace", "directories added with /add-dir", true},
	{"pr", "Pull request", "Workspace", "open PR or MR and its review state", true},
	{"context", "Context", "Gauges", "context window used", false},
	{"limit5h", "Limit 5h", "Gauges", "5-hour limit, Pro and Max", false},
	{"limit7d", "Limit 7d", "Gauges", "7-day limit, Pro and Max", false},
	{"spend", "Spend limit", "Gauges", "spend limit, behind a Claude apps gateway", true},
	{"tokens", "Tokens", "Context", "input and output tokens in the context", true},
	{"ctxsize", "Context size", "Context", "context window size, 200k or 1M", true},
	{"cache", "Prompt cache", "Context", "warm or cold, with the hit ratio", true},
	{"over200k", "Over 200k", "Context", "warning past 200k tokens", true},
	{"cost", "Cost", "Activity", "session cost in USD", false},
	{"lines", "Lines +/−", "Activity", "lines added and removed", false},
	{"reset5h", "Reset 5h", "Activity", "when the 5-hour limit resets", false},
	{"reset7d", "Reset 7d", "Activity", "when the 7-day limit resets", false},
	{"text", "Text", "Activity", "any text you type", false},
	{"spacer", "Spacer", "Layout", "an empty gap of a chosen width", false},
}

func isGauge(w string) bool {
	return w == "context" || w == "limit5h" || w == "limit7d" || w == "spend"
}

func (c Config) free(i int) bool { return i < len(c.Free) && c.Free[i] }

// alignIn pads v to width w, placing it left, center or right (like CSS justify-self).
func alignIn(v string, w int, align string) string {
	pad := w - ansi.StringWidth(v)
	switch align {
	case "right":
		return strings.Repeat(" ", pad) + v
	case "center":
		return strings.Repeat(" ", pad/2) + v + strings.Repeat(" ", pad-pad/2)
	}
	return v + strings.Repeat(" ", pad)
}

// spacerWidth: width reserved by a Spacer widget (Width, 4 by default).
func spacerWidth(c Cell) int {
	if c.Width == 0 {
		return 4
	}
	return c.Width
}

func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "dg-statuslines.json")
}

func loadConfig() (Config, error) {
	var cfg Config
	b, err := os.ReadFile(configPath())
	if err != nil {
		return cfg, err
	}
	return cfg, json.Unmarshal(b, &cfg)
}

type rateWindow struct {
	UsedPercentage *float64 `json:"used_percentage"`
	ResetsAt       *float64 `json:"resets_at"`
}

// Subset of the Claude Code session JSON used by the widgets.
// Fields: https://code.claude.com/docs/en/statusline#available-data
type session struct {
	Cwd         string `json:"cwd"`
	SessionName string `json:"session_name"`
	Version     string `json:"version"`
	Model       struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
	} `json:"model"`
	Workspace struct {
		CurrentDir  string   `json:"current_dir"`
		ProjectDir  string   `json:"project_dir"`
		AddedDirs   []string `json:"added_dirs"`
		GitWorktree string   `json:"git_worktree"`
		Repo        struct {
			Owner string `json:"owner"`
			Name  string `json:"name"`
		} `json:"repo"`
	} `json:"workspace"`
	Cost struct {
		TotalCostUSD       *float64 `json:"total_cost_usd"`
		TotalDurationMs    *float64 `json:"total_duration_ms"`
		TotalAPIDurationMs *float64 `json:"total_api_duration_ms"`
		TotalLinesAdded    *int     `json:"total_lines_added"`
		TotalLinesRemoved  *int     `json:"total_lines_removed"`
	} `json:"cost"`
	ContextWindow struct {
		UsedPercentage    *float64 `json:"used_percentage"`
		TotalInputTokens  *float64 `json:"total_input_tokens"`
		TotalOutputTokens *float64 `json:"total_output_tokens"`
		ContextWindowSize *float64 `json:"context_window_size"`
	} `json:"context_window"`
	Exceeds200k bool `json:"exceeds_200k_tokens"`
	PromptCache *struct {
		Warm     bool     `json:"warm"`
		HitRatio *float64 `json:"hit_ratio"`
	} `json:"prompt_cache"`
	FastMode bool `json:"fast_mode"`
	Effort   struct {
		Level string `json:"level"`
	} `json:"effort"`
	Thinking struct {
		Enabled bool `json:"enabled"`
	} `json:"thinking"`
	RateLimits struct {
		FiveHour   *rateWindow `json:"five_hour"`
		SevenDay   *rateWindow `json:"seven_day"`
		SpendLimit *rateWindow `json:"spend_limit"`
	} `json:"rate_limits"`
	OutputStyle struct {
		Name string `json:"name"`
	} `json:"output_style"`
	Vim struct {
		Mode string `json:"mode"`
	} `json:"vim"`
	Agent struct {
		Name string `json:"name"`
	} `json:"agent"`
	PR *struct {
		Number      int    `json:"number"`
		ReviewState string `json:"review_state"`
	} `json:"pr"`
	Worktree struct {
		Name string `json:"name"`
	} `json:"worktree"`
}

func (s session) dir() string {
	if s.Workspace.CurrentDir != "" {
		return s.Workspace.CurrentDir
	}
	return s.Cwd
}

func gitBranch(dir string) string {
	if dir == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", "-C", dir, "branch", "--show-current").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// paint colors s with a color resolved by colorSpec ("" = no color).
func paint(s, spec string) string {
	if s == "" || spec == "" {
		return s
	}
	if spec[0] == '#' {
		var r, g, b int
		fmt.Sscanf(spec, "#%02x%02x%02x", &r, &g, &b)
		return fmt.Sprintf("\x1b[38;2;%d;%d;%dm%s\x1b[0m", r, g, b, s)
	}
	return "\x1b[38;5;" + spec + "m" + s + "\x1b[0m"
}

func dim(s string) string {
	if s == "" {
		return s
	}
	return "\x1b[2m" + s + "\x1b[0m"
}

// colorSpec resolves a config color: palette name, 256 index ("178")
// or "#rrggbb". Returns the index or hex, "" if unknown.
func colorSpec(name string) string {
	if len(name) == 7 && name[0] == '#' {
		return name
	}
	if n, err := strconv.Atoi(name); err == nil && n >= 0 && n < 256 {
		return name
	}
	for _, c := range colors {
		if c.Name == name {
			return strconv.Itoa(c.Index)
		}
	}
	return ""
}

// bar returns the filled and empty parts of a w-character bar.
func bar(style string, pct float64, w int) (string, string) {
	steps := func(n int) int { return int(math.Round(pct * float64(w*n) / 100)) }
	// Sub-character styles: the last filled character is partial.
	partial := func(levels []rune, emptyCh string) (string, string) {
		n := len(levels) - 1
		t := steps(n)
		f := t / n
		s := strings.Repeat(string(levels[n]), f)
		if r := t % n; r > 0 && f < w {
			s += string(levels[r])
			f++
		}
		return s, strings.Repeat(emptyCh, w-f)
	}
	switch style {
	case "text":
		return "", ""
	case "fine":
		return partial([]rune(" ▏▎▍▌▋▊▉█"), " ")
	case "braille":
		return partial([]rune("⣀⣄⣤⣦⣶⣷⣿"), "⣀")
	}
	f := steps(1)
	switch style {
	case "slider": // rail filled up to the ● knob, which takes one cell
		f = min(f, w-1)
		return strings.Repeat("━", f) + "●", strings.Repeat("─", w-f-1)
	case "ramp": // height grows with position in the bar
		levels := []rune("▁▂▃▄▅▆▇█")
		var s strings.Builder
		for i := range f {
			s.WriteRune(levels[((i+1)*8+w-1)/w-1])
		}
		return s.String(), strings.Repeat("·", w-f)
	}
	ch, ok := map[string][2]string{
		"line": {"━", "─"}, "dots": {"●", "○"}, "ascii": {"#", "-"},
		"squares": {"■", "□"}, "diamonds": {"◆", "◇"}, "segments": {"▰", "▱"},
		"dashes": {"▬", "▭"}, "shade": {"▓", "░"},
	}[style]
	if !ok {
		ch = [2]string{"█", "░"}
	}
	return strings.Repeat(ch[0], f), strings.Repeat(ch[1], w-f)
}

func gauge(c Cell, used *float64, label string) string {
	if used == nil {
		return ""
	}
	u := math.Max(0, math.Min(100, *used))
	p := u
	if c.Mode == "left" {
		p = 100 - u
	}
	w := c.Width
	if w == 0 {
		w = 10
	}
	col := colorSpec(c.Color)
	if s, ok := schemes[c.Color]; ok {
		col = s[0]
		if u >= 80 {
			col = s[2]
		} else if u >= 50 {
			col = s[1]
		}
	}

	var parts []string
	if !c.NoLabel {
		parts = append(parts, dim(label))
	}
	fill, empty := bar(c.Bar, p, w)
	if fill+empty != "" {
		b := paint(fill, col) + dim(empty)
		for _, cp := range capStyles {
			if cp.ID == capsOf(c) {
				b = dim(cp.Left) + b + dim(cp.Right)
			}
		}
		parts = append(parts, b)
	}
	parts = append(parts, paint(fmt.Sprintf("%.0f%%", p), col))
	return strings.Join(parts, " ")
}

// modelVersion extracts the version from a model id:
// "claude-opus-5-5" -> "5.5", "claude-3-5-sonnet-20241022" -> "3.5", "claude-opus-4-1[1m]" -> "4.1".
func modelVersion(id string) string {
	id, _, _ = strings.Cut(id, "[")
	var nums []string
	for _, p := range strings.Split(id, "-") {
		if len(p) > 0 && len(p) <= 2 && strings.Trim(p, "0123456789") == "" {
			nums = append(nums, p) // dates (8 digits) are ignored
		}
	}
	return strings.Join(nums, ".")
}

func humanize(d time.Duration) string {
	s := max(int(d.Seconds()), 0)
	switch {
	case s >= 86400:
		return fmt.Sprintf("%dd%02dh", s/86400, s%86400/3600)
	case s >= 3600:
		return fmt.Sprintf("%dh%02dm", s/3600, s%3600/60)
	case s >= 60:
		return fmt.Sprintf("%dm", s/60)
	}
	return fmt.Sprintf("%ds", s)
}

// kilo shortens a token count: 950, 15.5k, 1M.
func kilo(n float64) string {
	switch {
	case n >= 1e6:
		return strings.TrimSuffix(fmt.Sprintf("%.1f", n/1e6), ".0") + "M"
	case n >= 1e3:
		return strings.TrimSuffix(fmt.Sprintf("%.1f", n/1e3), ".0") + "k"
	}
	return fmt.Sprintf("%.0f", n)
}

// ms formats a duration in milliseconds from the session JSON, "" if absent.
func ms(v *float64) string {
	if v == nil {
		return ""
	}
	return humanize(time.Duration(*v) * time.Millisecond)
}

func reset(c Cell, w *rateWindow, label string, withDay bool) string {
	if w == nil || w.ResetsAt == nil {
		return ""
	}
	t := time.Unix(int64(*w.ResetsAt), 0)
	at := t.Format("15:04")
	if withDay {
		at = t.Format("Mon 15:04")
	}
	return dim(label) + " " + paint(at, colorSpec(c.Color)) + " " + dim("("+humanize(time.Until(t))+")")
}

func usedOf(w *rateWindow) *float64 {
	if w == nil {
		return nil
	}
	if w.UsedPercentage == nil {
		return new(float64) // window present without a percentage: 0 %
	}
	return w.UsedPercentage
}

// cellValue returns a cell's colored text, "" if the data is missing.
func cellValue(c Cell, s session) string {
	col := colorSpec(c.Color)
	switch c.Widget {
	case "model":
		name := s.Model.DisplayName
		if name == "" {
			return ""
		}
		// Only append the version if the display name doesn't already contain one.
		if v := modelVersion(s.Model.ID); c.Version && v != "" && !strings.ContainsAny(name, "0123456789") {
			name += " " + v
		}
		if c.Icon {
			name = "✳ " + name
		}
		return paint(name, col)
	case "dir":
		d := s.dir()
		if d == "" {
			return ""
		}
		if !c.Full {
			return paint(filepath.Base(d), col)
		}
		if home, _ := os.UserHomeDir(); home != "" && (d == home || strings.HasPrefix(d, home+"/")) {
			d = "~" + strings.TrimPrefix(d, home)
		}
		return paint(d, col)
	case "git":
		return paint(gitBranch(s.dir()), col)
	case "userhost":
		u, err := user.Current()
		h, _ := os.Hostname()
		if err != nil || h == "" {
			return ""
		}
		h, _, _ = strings.Cut(h, ".")
		return paint(u.Username+"@"+h, col)
	case "cost":
		if s.Cost.TotalCostUSD != nil {
			return paint(fmt.Sprintf("$%.2f", *s.Cost.TotalCostUSD), col)
		}
	case "lines":
		if s.Cost.TotalLinesAdded == nil || s.Cost.TotalLinesRemoved == nil {
			return ""
		}
		add, del := "2", "1" // green / red unless a color is chosen
		if col != "" {
			add, del = col, col
		}
		return paint(fmt.Sprintf("+%d", *s.Cost.TotalLinesAdded), add) + " " +
			paint(fmt.Sprintf("−%d", *s.Cost.TotalLinesRemoved), del)
	case "context":
		return gauge(c, s.ContextWindow.UsedPercentage, "ctx")
	case "limit5h":
		return gauge(c, usedOf(s.RateLimits.FiveHour), "5h")
	case "limit7d":
		return gauge(c, usedOf(s.RateLimits.SevenDay), "7d")
	case "reset5h":
		return reset(c, s.RateLimits.FiveHour, "5h ↻", false)
	case "reset7d":
		return reset(c, s.RateLimits.SevenDay, "7d ↻", true)
	case "text":
		return paint(c.Text, col)
	case "spend":
		return gauge(c, usedOf(s.RateLimits.SpendLimit), "$")
	case "session":
		return paint(s.SessionName, col)
	case "duration":
		if d := ms(s.Cost.TotalDurationMs); d != "" {
			return dim("⏱") + " " + paint(d, col)
		}
	case "apitime":
		if d := ms(s.Cost.TotalAPIDurationMs); d != "" {
			return dim("api") + " " + paint(d, col)
		}
	case "version":
		if s.Version != "" {
			return paint("v"+s.Version, col)
		}
	case "agent":
		return paint(s.Agent.Name, col)
	case "style":
		if s.OutputStyle.Name != "default" {
			return paint(s.OutputStyle.Name, col)
		}
	case "vim":
		return paint(s.Vim.Mode, col)
	case "effort":
		if s.Effort.Level != "" {
			return dim("effort") + " " + paint(s.Effort.Level, col)
		}
	case "fast":
		if s.FastMode {
			return paint("↯ fast", col)
		}
	case "thinking":
		if s.Thinking.Enabled {
			return paint("thinking", col)
		}
	case "project":
		if s.Workspace.ProjectDir != "" {
			return paint(filepath.Base(s.Workspace.ProjectDir), col)
		}
	case "repo":
		if r := s.Workspace.Repo; r.Owner != "" && r.Name != "" {
			return paint(r.Owner+"/"+r.Name, col)
		}
	case "worktree":
		wt := s.Workspace.GitWorktree
		if wt == "" {
			wt = s.Worktree.Name
		}
		if wt != "" {
			return dim("wt") + " " + paint(wt, col)
		}
	case "added":
		switch n := len(s.Workspace.AddedDirs); {
		case n == 1:
			return paint("+1 dir", col)
		case n > 1:
			return paint(fmt.Sprintf("+%d dirs", n), col)
		}
	case "pr":
		if s.PR == nil || s.PR.Number == 0 {
			return ""
		}
		v := paint(fmt.Sprintf("#%d", s.PR.Number), col)
		if s.PR.ReviewState != "" {
			v += " " + dim(strings.ReplaceAll(s.PR.ReviewState, "_", " "))
		}
		return v
	case "tokens":
		cw := s.ContextWindow
		if cw.TotalInputTokens == nil || cw.TotalOutputTokens == nil {
			return ""
		}
		return dim("in") + " " + paint(kilo(*cw.TotalInputTokens), col) + " " + dim("out") + " " + paint(kilo(*cw.TotalOutputTokens), col)
	case "ctxsize":
		if n := s.ContextWindow.ContextWindowSize; n != nil && *n > 0 {
			return paint(kilo(*n), col)
		}
	case "cache":
		pc := s.PromptCache
		if pc == nil {
			return ""
		}
		state := "cold"
		if pc.Warm {
			state = "warm"
		}
		v := dim("cache") + " " + paint(state, col)
		if pc.HitRatio != nil {
			v += " " + dim(fmt.Sprintf("%.0f%%", *pc.HitRatio*100))
		}
		return v
	case "over200k":
		if s.Exceeds200k {
			return paint("⚠ >200k", col)
		}
	}
	return ""
}

// render builds the status line. Invalid JSON => empty fields, never an error.
// Colors are raw ANSI codes: stdout is not a TTY when Claude Code
// captures the output, so lipgloss would strip them.
func render(input []byte, cfg Config) string {
	var s session
	_ = json.Unmarshal(input, &s)
	pad := " "
	if cfg.AirCols {
		pad = "   "
	}
	// Every separator is one character between two pads, so all have the same width
	// and columns stay aligned whatever separator each cell picks.
	mkSep := func(id string) string {
		switch id {
		case "":
			id = "│"
		case "space":
			id = " "
		}
		return pad + id + pad
	}
	global := mkSep(cfg.Sep)
	sepAfter := func(c Cell) string {
		if c.Sep == "" {
			return global
		}
		return mkSep(c.Sep)
	}
	sepW := ansi.StringWidth(global)

	// CSS-like grid: each column takes the width of its widest cell.
	// An empty cell (or a widget without data) leaves a blank as wide as its
	// column, with no separator; a column empty on every row disappears,
	// unless a Spacer widget forces a width on it.
	// A row off the grid neither sets nor follows the column widths: its cells take
	// their own width, so a long value there pushes nothing on the other rows.
	vals := make([][]string, len(cfg.Rows))
	cellW := func(c Cell, v string) int {
		if c.Widget == "spacer" {
			return max(ansi.StringWidth(v), spacerWidth(c))
		}
		return ansi.StringWidth(v)
	}
	var widths []int
	for i, row := range cfg.Rows {
		vals[i] = make([]string, len(row))
		for k, c := range row {
			vals[i][k] = cellValue(c, s)
			if cfg.free(i) {
				continue
			}
			for len(widths) <= k {
				widths = append(widths, 0)
			}
			widths[k] = max(widths[k], cellW(c, vals[i][k]))
		}
	}

	var lines []string
	for i, row := range cfg.Rows {
		var b strings.Builder
		any, prev, first := false, false, true
		prevK := -1 // last column written: its cell picks the separator that follows it
		rowW := widths
		if cfg.free(i) {
			rowW = make([]int, len(row))
			for k, c := range row {
				rowW[k] = cellW(c, vals[i][k])
			}
		}
		for k, w := range rowW {
			if w == 0 {
				continue
			}
			v := ""
			if k < len(vals[i]) {
				v = vals[i][k]
			}
			if !first {
				if prev && v != "" {
					b.WriteString(sepAfter(row[prevK]))
				} else {
					b.WriteString(strings.Repeat(" ", sepW))
				}
			}
			first, prev, prevK = false, v != "", k
			any = any || prev
			align := ""
			if k < len(cfg.Rows[i]) {
				align = cfg.Rows[i][k].Align
			}
			b.WriteString(alignIn(v, w, align))
		}
		if !any {
			continue // row without data: skip the row and its gap
		}
		lines = append(lines, strings.TrimRight(b.String(), " "))
		if i < len(cfg.Gaps) && cfg.Gaps[i] {
			// Zero-width space rather than an empty line, as in the old statusline-command.sh.
			lines = append(lines, "\u200b")
		}
	}
	return strings.Join(frame(lines, cfg), "\n")
}

// frame wraps the lines in a border as wide as the longest one.
// Blank lines (zero-width space) become empty lines inside the frame.
func frame(lines []string, cfg Config) []string {
	for _, b := range borders {
		if b.ID != cfg.Border || len(lines) == 0 {
			continue
		}
		bc := colorSpec(cfg.BorderColor)
		edge := func(s string) string {
			if bc == "" {
				return dim(s)
			}
			return paint(s, bc)
		}
		w := 0
		for _, l := range lines {
			w = max(w, ansi.StringWidth(l))
		}
		bottom, right := b.B, b.R
		if bottom == "" {
			bottom = b.H
		}
		if right == "" {
			right = b.V
		}
		out := []string{edge(b.TL + strings.Repeat(b.H, w+2) + b.TR)}
		for _, l := range lines {
			if l == "\u200b" {
				l = ""
			}
			out = append(out, edge(b.V)+" "+l+strings.Repeat(" ", w-ansi.StringWidth(l))+" "+edge(right))
		}
		return append(out, edge(b.BL+strings.Repeat(bottom, w+2)+b.BR))
	}
	return lines
}
