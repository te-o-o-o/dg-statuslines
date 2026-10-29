package main

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone"
)

const (
	minW = 60 // below this: "terminal too small" state
	minH = 20
)

var tabNames = []string{"Layout", "Style", "Install"}

type pos struct{ r, c int }

type drag struct {
	from *pos // nil: comes from the palette
	cell Cell
}

// toastDone clears toast n, unless it has been replaced since.
type toastDone int

type model struct {
	cfg     Config
	saved   string // saved config (JSON), for the "unsaved" indicator
	sample  []byte
	branch  string // branch shown in the preview (computed once at startup)
	tab     int
	w, h    int
	sel     *pos
	selSep  *pos   // separator slot selected: the one after this cell
	section string // open settings section (Color, Bar…)
	scroll  int    // scroll offset of the content below the preview
	rng     *rand.Rand
	rolls   []Config // random layouts drawn so far, to step back through them
	rollIdx int
	editing bool // typing the free text of the selected cell
	drag    *drag
	hover   string // zone hovered during the drag
	mx, my  int
	toast   string
	toastN  int

	// Widget catalogue window.
	cat       bool
	catQuery  string // filter typed in the window
	catSel    int    // selected widget among those matching
	catScroll int

	// Install tab, recomputed each time the tab opens.
	profiles []profile
}

var (
	title     = lipgloss.NewStyle().Bold(true)
	keyStyle  = lipgloss.NewStyle().Bold(true)
	okStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	warnStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	errStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
)

func edit() error {
	cfg, err := loadConfig()
	saved := ""
	if err != nil || len(cfg.Rows) == 0 {
		cfg = Config{Rows: [][]Cell{make([]Cell, 4), make([]Cell, 4)}}
	}
	// All rows have the same number of columns.
	rows := cfg.Rows
	cfg.Rows = nil
	cfg = grid("", cfg, rows...)
	// No widget list chosen yet: the non-extra widgets.
	if cfg.Palette == nil {
		for _, w := range widgets {
			if !w.Extra {
				cfg.Palette = append(cfg.Palette, w.ID)
			}
		}
	}
	if err == nil {
		saved = configJSON(cfg)
	}

	cwd, _ := os.Getwd()
	now := time.Now().Unix()
	sample, _ := json.Marshal(map[string]any{
		"model":        map[string]any{"id": "claude-opus-5-5", "display_name": "Opus"},
		"session_name": "fix-login-flow",
		"version":      "2.1.90",
		"workspace": map[string]any{"current_dir": cwd, "project_dir": cwd, "added_dirs": []string{"../shared"},
			"git_worktree": "feature-xyz", "repo": map[string]any{"owner": "acme", "name": "webshop"}},
		"cost": map[string]any{"total_cost_usd": 0.42, "total_lines_added": 156, "total_lines_removed": 23,
			"total_duration_ms": 4320000, "total_api_duration_ms": 750000},
		"context_window": map[string]any{"used_percentage": 23, "total_input_tokens": 46000,
			"total_output_tokens": 1200, "context_window_size": 200000},
		"exceeds_200k_tokens": true,
		"prompt_cache":        map[string]any{"warm": true, "hit_ratio": 0.91},
		"fast_mode":           true,
		"effort":              map[string]any{"level": "high"},
		"thinking":            map[string]any{"enabled": true},
		"output_style":        map[string]any{"name": "Explanatory"},
		"vim":                 map[string]any{"mode": "NORMAL"},
		"agent":               map[string]any{"name": "reviewer"},
		"pr":                  map[string]any{"number": 1234, "review_state": "pending"},
		"rate_limits": map[string]any{
			"five_hour":   map[string]any{"used_percentage": 62.5, "resets_at": now + 2*3600 + 12*60},
			"seven_day":   map[string]any{"used_percentage": 85.2, "resets_at": now + 3*86400 + 4*3600},
			"spend_limit": map[string]any{"used_percentage": 41, "resets_at": now + 12*86400},
		},
	})

	branch := gitBranch(cwd)
	if branch == "" {
		branch = "main" // outside a git repo: sample value so the widget stays visible
	}

	darkTerminal = lipgloss.HasDarkBackground()
	applyUI(cfg.UI)
	zone.NewGlobal()
	_, err = tea.NewProgram(model{cfg: cfg, saved: saved, sample: sample, branch: branch}, tea.WithAltScreen(), tea.WithMouseAllMotion()).Run()
	return err
}

func configJSON(cfg Config) string {
	b, _ := json.MarshalIndent(cfg, "", "  ")
	return string(b)
}

func (m model) dirty() bool { return configJSON(m.cfg) != m.saved }

func (m model) Init() tea.Cmd { return nil }

func cellID(r, c int) string { return fmt.Sprintf("cell:%d:%d", r, c) }

func parseCell(id string) (pos, bool) {
	var p pos
	_, err := fmt.Sscanf(id, "cell:%d:%d", &p.r, &p.c)
	return p, err == nil
}

// marked: the zone ids drawn since the last View. A zone's content is built before
// the zone itself, so nested zones (pal:… inside palette) come before their container.
var marked []string

// mark is zone.Mark, remembering the id for hit.
func mark(id, s string) string {
	marked = append(marked, id)
	return zone.Mark(id, s)
}

// hit returns the id of the zone under the mouse ("" if none).
func (m model) hit(msg tea.MouseMsg) string {
	for _, id := range marked {
		if zone.Get(id).InBounds(msg) {
			return id
		}
	}
	return ""
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
	case toastDone:
		if int(msg) == m.toastN {
			m.toast = ""
		}
	case tea.KeyMsg:
		return m.key(msg)
	case tea.MouseMsg:
		m.mx, m.my = msg.X, msg.Y
		if m.cat {
			return m.catMouse(msg)
		}
		hit := m.hit(msg)
		switch msg.Action {
		case tea.MouseActionPress:
			switch msg.Button {
			case tea.MouseButtonLeft:
				return m.press(hit)
			case tea.MouseButtonWheelUp:
				m.scrollBy(-3)
			case tea.MouseButtonWheelDown:
				m.scrollBy(3)
			}
		case tea.MouseActionMotion:
			m.hover = hit // hover (pink borders) and drag-and-drop target
		case tea.MouseActionRelease:
			if m.drag != nil {
				m.drop(hit)
			}
			m.drag, m.hover = nil, ""
		}
	}
	return m, nil
}

// notify shows a toast (success: 3 s; error: until the next one).
func (m *model) notify(msg string, err bool) tea.Cmd {
	m.toastN++
	if err {
		m.toast = errStyle.Render("✗ ") + msg
		return nil
	}
	m.toast = okStyle.Render("✓ ") + msg
	n := m.toastN
	return tea.Tick(3*time.Second, func(time.Time) tea.Msg { return toastDone(n) })
}

func (m *model) setTab(t int) {
	m.tab, m.editing, m.scroll = t, false, 0
	if t == 2 {
		m.profiles = nil
		for _, d := range profileDirs() {
			m.profiles = append(m.profiles, planInstall(d))
		}
	}
}

func (m *model) save() tea.Cmd {
	s := configJSON(m.cfg)
	if err := os.WriteFile(configPath(), []byte(s+"\n"), 0o644); err != nil {
		return m.notify("Cannot save: "+err.Error(), true)
	}
	m.saved = s
	return m.notify("Saved", false)
}

func (m model) press(hit string) (tea.Model, tea.Cmd) {
	m.editing = false
	if p, ok := parseCell(hit); ok {
		m.selSep = nil
		if c := m.cfg.Rows[p.r][p.c]; c.Widget != "" {
			m.drag = &drag{from: &p, cell: c}
		} else {
			m.sel = &p
		}
		return m, nil
	}
	switch {
	case strings.HasPrefix(hit, "sepslot:"):
		var p pos
		fmt.Sscanf(hit, "sepslot:%d:%d", &p.r, &p.c)
		m.sel, m.selSep = nil, &p
	case strings.HasPrefix(hit, "cellsep:") && m.selSep != nil:
		m.cfg.Rows[m.selSep.r][m.selSep.c].Sep = strings.TrimPrefix(hit, "cellsep:")
	case hit == "insp:text":
		m.editing = true
	case strings.HasPrefix(hit, "sec:"):
		m.section = strings.TrimPrefix(hit, "sec:")
	case strings.HasPrefix(hit, "color:"), strings.HasPrefix(hit, "opt:"):
		m.apply(hit)
	case strings.HasPrefix(hit, "tab:"):
		t, _ := strconv.Atoi(strings.TrimPrefix(hit, "tab:"))
		m.setTab(t)
	case strings.HasPrefix(hit, "btn:install:"):
		i, _ := strconv.Atoi(strings.TrimPrefix(hit, "btn:install:"))
		return m, m.install(i)
	case hit == "btn:catalogue":
		m.cat = true
	case strings.HasPrefix(hit, "pal:"):
		m.drag = &drag{cell: Cell{Widget: strings.TrimPrefix(hit, "pal:")}}
	case strings.HasPrefix(hit, "free:"):
		r, _ := strconv.Atoi(strings.TrimPrefix(hit, "free:"))
		for len(m.cfg.Free) <= r {
			m.cfg.Free = append(m.cfg.Free, false)
		}
		m.cfg.Free[r] = !m.cfg.Free[r]
	case strings.HasPrefix(hit, "gap:"):
		r, _ := strconv.Atoi(strings.TrimPrefix(hit, "gap:"))
		m.setGap(r, !m.gap(r))
	case hit == "btn:airrows":
		on := !m.allGaps()
		for r := range len(m.cfg.Rows) - 1 {
			m.setGap(r, on)
		}
	case hit == "btn:aircols":
		m.cfg.AirCols = !m.cfg.AirCols
	case hit == "preset:random":
		return m, m.roll()
	case hit == "preset:prev":
		return m, m.prevRoll()
	case strings.HasPrefix(hit, "preset:"):
		for _, p := range presets {
			if "preset:"+p.ID == hit {
				m.replace(p.build())
				return m, m.notify(p.Label+" applied · s to save", false)
			}
		}
	case strings.HasPrefix(hit, "ui:"):
		m.cfg.UI = strings.TrimPrefix(hit, "ui:")
		applyUI(m.cfg.UI)
	case strings.HasPrefix(hit, "theme:"):
		if t, ok := themeByID(strings.TrimPrefix(hit, "theme:")); ok {
			recolor(&m.cfg, t)
		}
	case strings.HasPrefix(hit, "sep:"):
		m.cfg.Sep = strings.TrimPrefix(hit, "sep:")
	case strings.HasPrefix(hit, "border:"):
		m.cfg.Border = strings.TrimPrefix(hit, "border:")
	case strings.HasPrefix(hit, "bcolor:"):
		m.cfg.BorderColor = strings.TrimPrefix(hit, "bcolor:")
	case hit == "btn:+row":
		m.cfg.Rows = append(m.cfg.Rows, make([]Cell, len(m.cfg.Rows[0])))
	case hit == "btn:-row" && len(m.cfg.Rows) > 1:
		m.cfg.Rows = m.cfg.Rows[:len(m.cfg.Rows)-1]
		m.cfg.Gaps = m.cfg.Gaps[:min(len(m.cfg.Gaps), len(m.cfg.Rows))]
		m.cfg.Free = m.cfg.Free[:min(len(m.cfg.Free), len(m.cfg.Rows))]
	case hit == "btn:+col" && len(m.cfg.Rows[0]) < 8:
		for i := range m.cfg.Rows {
			m.cfg.Rows[i] = append(m.cfg.Rows[i], Cell{})
		}
	case hit == "btn:-col" && len(m.cfg.Rows[0]) > 1:
		for i := range m.cfg.Rows {
			m.cfg.Rows[i] = m.cfg.Rows[i][:len(m.cfg.Rows[i])-1]
		}
	}
	if m.sel != nil && (m.sel.r >= len(m.cfg.Rows) || m.sel.c >= len(m.cfg.Rows[0])) {
		m.sel = nil
	}
	if m.selSep != nil && (m.selSep.r >= len(m.cfg.Rows) || m.selSep.c >= len(m.cfg.Rows[0])-1) {
		m.selSep = nil
	}
	return m, nil
}

// apply applies a choice to the selected cell (list or inspector).
func (m *model) apply(id string) {
	if m.sel == nil {
		return
	}
	c := &m.cfg.Rows[m.sel.r][m.sel.c]
	switch {
	case strings.HasPrefix(id, "color:"):
		c.Color = strings.TrimPrefix(id, "color:")
	case strings.HasPrefix(id, "opt:"):
		k, v, _ := strings.Cut(strings.TrimPrefix(id, "opt:"), ":")
		switch k {
		case "bar":
			c.Bar = v
		case "mode":
			c.Mode = v
		case "width":
			c.Width, _ = strconv.Atoi(v)
		case "label":
			c.NoLabel = v == "off"
		case "caps":
			c.Caps = v
		case "version":
			c.Version = v == "yes"
		case "icon":
			c.Icon = v == "yes"
		case "align":
			c.Align = v
		case "full":
			c.Full = v == "yes"
		}
	}
}

// moveSel moves the selected cell with the keyboard.
func (m *model) moveSel(dr, dc int) {
	if m.sel == nil {
		m.sel, m.selSep = &pos{0, 0}, nil
		return
	}
	r := min(max(m.sel.r+dr, 0), len(m.cfg.Rows)-1)
	c := min(max(m.sel.c+dc, 0), len(m.cfg.Rows[0])-1)
	m.sel = &pos{r, c}
}

func (m *model) drop(hit string) {
	d := m.drag
	if t, ok := parseCell(hit); ok {
		if d.from == nil {
			m.cfg.Rows[t.r][t.c] = d.cell // palette -> cell (replaces)
			if th, ok := themeByID(m.cfg.Theme); ok {
				m.cfg.Rows[t.r][t.c].Color = th.color(d.cell.Widget)
			}
		} else if *d.from != t {
			// cell -> cell: move, or swap if the target is taken
			rows := m.cfg.Rows
			rows[t.r][t.c], rows[d.from.r][d.from.c] = rows[d.from.r][d.from.c], rows[t.r][t.c]
		}
		m.sel, m.selSep = &t, nil
		m.editing = m.cfg.Rows[t.r][t.c].Widget == "text"
		return
	}
	if (hit == "palette" || strings.HasPrefix(hit, "pal:")) && d.from != nil {
		m.cfg.Rows[d.from.r][d.from.c] = Cell{} // cell -> palette: remove
		m.sel = nil
	}
}

func (m model) key(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}
	if m.cat {
		return m.catKey(msg)
	}
	if m.editing && m.sel != nil {
		c := &m.cfg.Rows[m.sel.r][m.sel.c]
		switch msg.Type {
		case tea.KeyEnter, tea.KeyEsc:
			m.editing = false
		case tea.KeyBackspace:
			if r := []rune(c.Text); len(r) > 0 {
				c.Text = string(r[:len(r)-1])
			}
		case tea.KeyRunes, tea.KeySpace:
			c.Text += string(msg.Runes)
		}
		return m, nil
	}
	if m.tab == 0 {
		switch msg.String() {
		case "up":
			m.moveSel(-1, 0)
			return m, nil
		case "down":
			m.moveSel(1, 0)
			return m, nil
		case "left":
			m.moveSel(0, -1)
			return m, nil
		case "right":
			m.moveSel(0, 1)
			return m, nil
		case "enter":
			if c := m.selCell(); c != nil && c.Widget == "text" {
				m.editing = true
			}
			return m, nil
		case "+":
			m.cat = true
			return m, nil
		case "x":
			if m.sel != nil {
				m.cfg.Rows[m.sel.r][m.sel.c] = Cell{}
			}
			return m, nil
		case "[", "]":
			if c := m.selCell(); c != nil && c.Widget != "" {
				secs := sections(c)
				i := 0
				for k, sec := range secs {
					if sec == m.curSection() {
						i = k
					}
				}
				d := 1
				if msg.String() == "[" {
					d = -1
				}
				m.section = secs[(i+d+len(secs))%len(secs)]
			}
			return m, nil
		}
	}
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "1", "2", "3":
		m.setTab(int(msg.Runes[0] - '1'))
	case "tab":
		m.setTab((m.tab + 1) % len(tabNames))
	case "shift+tab":
		m.setTab((m.tab + len(tabNames) - 1) % len(tabNames))
	case "esc":
		m.sel, m.selSep = nil, nil
	case "pgdown":
		m.scrollBy(m.bodyRoom() - 2)
	case "pgup":
		m.scrollBy(-(m.bodyRoom() - 2))
	case "r":
		return m, m.roll()
	case "R":
		return m, m.prevRoll()
	case "s":
		return m, m.save()
	case "i":
		m.setTab(2)
	case "enter":
		if m.tab == 2 {
			return m, m.install(m.currentProfile())
		}
	}
	return m, nil
}

func label(id string) string {
	for _, w := range widgets {
		if w.ID == id {
			return w.Label
		}
	}
	return id
}

func colorOf(name string) lipgloss.Style {
	if spec := colorSpec(name); spec != "" {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(spec))
	}
	return lipgloss.NewStyle()
}

func toggleStyle(on bool) lipgloss.Style {
	if on {
		return button.Background(accent).Foreground(onAccent)
	}
	return button
}

func toggle(id, label string, on bool) string {
	return mark(id, toggleStyle(on).Render(label))
}

func (m model) gap(r int) bool { return r < len(m.cfg.Gaps) && m.cfg.Gaps[r] }

func (m *model) setGap(r int, on bool) {
	for len(m.cfg.Gaps) <= r {
		m.cfg.Gaps = append(m.cfg.Gaps, false)
	}
	m.cfg.Gaps[r] = on
}

// allGaps: a blank line between every pair of rows.
func (m model) allGaps() bool {
	for r := range len(m.cfg.Rows) - 1 {
		if !m.gap(r) {
			return false
		}
	}
	return len(m.cfg.Rows) > 1
}

// width: usable width, 80 until the terminal reports its size.
func (m model) width() int {
	if m.w == 0 {
		return 80
	}
	return m.w
}

// height: terminal height, 24 until the terminal reports its size.
func (m model) height() int {
	if m.h == 0 {
		return 24
	}
	return m.h
}

// titledBox draws a gray box of width w with a title in the top edge.
func titledBox(name, content string, w int) string {
	edge := lipgloss.NewStyle().Foreground(edgeColor)
	inner := w - 4
	top := "╭─ " + name + " " + strings.Repeat("─", max(0, w-len([]rune(name))-5)) + "╮"
	lines := []string{edge.Render(top)}
	for _, l := range strings.Split(content, "\n") {
		l = ansi.Truncate(l, inner, "…")
		lines = append(lines, edge.Render("│")+" "+l+strings.Repeat(" ", max(0, inner-ansi.StringWidth(l)))+" "+edge.Render("│"))
	}
	lines = append(lines, edge.Render("╰"+strings.Repeat("─", w-2)+"╯"))
	return strings.Join(lines, "\n")
}

func (m model) header() string {
	left := " " + title.Render("dg-statuslines") + "   "
	for i, n := range tabNames {
		st := faint
		if i == m.tab {
			st = lipgloss.NewStyle().Bold(true).Underline(true).Foreground(accent)
		}
		left += mark(fmt.Sprintf("tab:%d", i), st.Render(n)) + "   "
	}
	right := m.toast
	if right == "" {
		if m.dirty() {
			right = warnStyle.Render("● unsaved")
		} else {
			right = faint.Render("✓ saved")
		}
	}
	gap := max(1, m.width()-ansi.StringWidth(left)-ansi.StringWidth(right)-1)
	return left + strings.Repeat(" ", gap) + right
}

func (m model) previewBox() string {
	// Git cells become fixed text: no git subprocess on every frame.
	pcfg := cloneConfig(m.cfg)
	for _, row := range pcfg.Rows {
		for i, c := range row {
			if c.Widget == "git" {
				row[i].Widget, row[i].Text = "text", m.branch // keeps color, alignment and separator
			}
		}
	}
	preview := render(m.sample, pcfg)
	if preview == "" {
		preview = faint.Render("Drag a widget onto the grid to get started.")
	}
	return titledBox("Preview", preview, m.width()-2)
}

// hints renders a help line "key action · key action".
func hints(pairs ...string) string {
	var parts []string
	for i := 0; i+1 < len(pairs); i += 2 {
		parts = append(parts, keyStyle.Render(pairs[i])+" "+faint.Render(pairs[i+1]))
	}
	return " " + strings.Join(parts, faint.Render(" · "))
}

func (m model) footer() string {
	switch {
	case m.editing:
		return hints("enter", "finish typing", "⌫", "erase")
	case m.tab == 0 && m.sel != nil:
		return hints("←↑↓→", "cell", "[ ]", "section", "x", "remove", "esc", "back", "s", "save")
	case m.tab == 0:
		return hints("drag", "place", "click", "edit", "+", "more widgets", "1-3", "tab", "s", "save", "q", "quit")
	case m.tab == 1:
		return hints("click", "choose", "1-3", "tab", "s", "save", "q", "quit")
	}
	return hints("enter", "install in this terminal's profile", "1-3", "tab", "q", "quit")
}

// labelW: width of the label column, shared by all settings.
const labelW = 14

// field lays out a setting: label in a fixed column, controls on the right.
func field(name, content string) string {
	return lipgloss.JoinHorizontal(lipgloss.Top, faint.Width(labelW).Render(name), content)
}

// bodyLines: the tab content, line by line, before scrolling.
func (m model) bodyLines() []string {
	var body string
	switch m.tab {
	case 0:
		body = m.layoutTab()
	case 1:
		body = m.styleTab()
	default:
		body = m.installTab()
	}
	// Indent by 2 columns to align the content with the inside of the preview.
	return strings.Split(lipgloss.NewStyle().PaddingLeft(2).Render(body), "\n")
}

// bodyRoom: lines available for the content, between the preview and the help.
func (m model) bodyRoom() int {
	return max(3, m.height()-3-lipgloss.Height(m.previewBox())-1)
}

func (m *model) scrollBy(d int) {
	m.scroll = min(max(0, m.scroll+d), max(0, len(m.bodyLines())-m.bodyRoom()))
}

func (m model) View() string {
	if m.w > 0 && (m.w < minW || m.h < minH) {
		return fmt.Sprintf("\n  Terminal too small: %d×%d minimum (currently %d×%d).\n  Enlarge the window, or press %s to quit.",
			minW, minH, m.w, m.h, keyStyle.Render("q"))
	}

	marked = nil
	// Fixed header and preview, scrolling content, help at the bottom.
	top := strings.Join([]string{m.header(), "", m.previewBox(), ""}, "\n")
	bl := m.bodyLines()
	room := m.bodyRoom()
	off := min(m.scroll, max(0, len(bl)-room))
	shown := bl[off:min(len(bl), off+room)]
	for len(shown) < room {
		shown = append(shown, "")
	}
	help := m.footer()
	if more := len(bl) - off - room; more > 0 || off > 0 {
		help = faint.Render(fmt.Sprintf(" ↕ %d/%d ·", off+room, len(bl))) + help
	}
	// A single zone.Scan for the whole screen: each Scan clears the previous one's zones.
	view := zone.Scan(top + "\n" + strings.Join(shown, "\n") + "\n" + help)
	lines := strings.Split(view, "\n")
	for i, l := range lines {
		lines[i] = ansi.Truncate(l, m.width(), "")
	}
	view = strings.Join(lines, "\n")

	if m.cat {
		box, x := m.catBox()
		view = overlay(view, box, x, catTop)
	}
	if m.drag != nil {
		view = overlay(view, dragLabel.Render("⠿ "+label(m.drag.cell.Widget)), m.mx+2, m.my)
	}
	return view
}

// overlay draws fg over bg with its top-left corner at column x, line y
// (label following the cursor, catalogue window).
func overlay(bg, fg string, x, y int) string {
	lines := strings.Split(bg, "\n")
	for i, f := range strings.Split(fg, "\n") {
		if y+i < 0 || y+i >= len(lines) {
			continue
		}
		l := lines[y+i]
		if w := ansi.StringWidth(l); w < x {
			l += strings.Repeat(" ", x-w)
		}
		lines[y+i] = ansi.Truncate(l, x, "") + "\x1b[0m" + f + "\x1b[0m" + ansi.TruncateLeft(l, x+ansi.StringWidth(f), "")
	}
	return strings.Join(lines, "\n")
}
