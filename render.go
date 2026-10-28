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
}

// Widget id -> label shown in the builder (palette order).
var widgets = []struct{ ID, Label string }{
	{"model", "Model"},
	{"dir", "Directory"},
	{"git", "Git branch"},
	{"userhost", "user@host"},
	{"cost", "Cost"},
	{"lines", "Lines +/−"},
	{"context", "Context"},
	{"limit5h", "Limit 5h"},
	{"limit7d", "Limit 7d"},
	{"reset5h", "Reset 5h"},
	{"reset7d", "Reset 7d"},
	{"text", "Text"},
	{"spacer", "Spacer"},
}

func isGauge(w string) bool { return w == "context" || w == "limit5h" || w == "limit7d" }

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
	return filepath.Join(home, ".claude", "statusline-builder.json")
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
type session struct {
	Cwd   string `json:"cwd"`
	Model struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
	} `json:"model"`
	Workspace struct {
		CurrentDir string `json:"current_dir"`
	} `json:"workspace"`
	Cost struct {
		TotalCostUSD      *float64 `json:"total_cost_usd"`
		TotalLinesAdded   *int     `json:"total_lines_added"`
		TotalLinesRemoved *int     `json:"total_lines_removed"`
	} `json:"cost"`
	ContextWindow struct {
		UsedPercentage *float64 `json:"used_percentage"`
	} `json:"context_window"`
	RateLimits struct {
		FiveHour *rateWindow `json:"five_hour"`
		SevenDay *rateWindow `json:"seven_day"`
	} `json:"rate_limits"`
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
