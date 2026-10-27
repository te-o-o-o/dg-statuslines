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
	cellWidth = 14
	minW      = 60 // below this: "terminal too small" state
	minH      = 20
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
	selRow  *int   // row selected via its gutter (L1, L2…)
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

	// Install tab, recomputed each time the tab opens.
	plan           []byte
	oldCmd, newCmd string
	planErr        error
}

var (
	title     = lipgloss.NewStyle().Bold(true)
	keyStyle  = lipgloss.NewStyle().Bold(true)
	okStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	warnStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	errStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	box       = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
)

func edit() error {
	cfg, err := loadConfig()
	saved := ""
	if err != nil || len(cfg.Rows) == 0 {
		cfg = Config{Rows: [][]Cell{make([]Cell, 4), make([]Cell, 4)}}
	}
	// All rows have the same number of columns.
	cols := 1
	for _, r := range cfg.Rows {
		cols = max(cols, len(r))
	}
	for i := range cfg.Rows {
		for len(cfg.Rows[i]) < cols {
			cfg.Rows[i] = append(cfg.Rows[i], Cell{})
		}
	}
	if err == nil {
		saved = configJSON(cfg)
	}

	cwd, _ := os.Getwd()
	now := time.Now().Unix()
	sample, _ := json.Marshal(map[string]any{
		"model":          map[string]any{"id": "claude-opus-5-5", "display_name": "Opus"},
		"workspace":      map[string]any{"current_dir": cwd},
		"cost":           map[string]any{"total_cost_usd": 0.42, "total_lines_added": 156, "total_lines_removed": 23},
		"context_window": map[string]any{"used_percentage": 23},
		"rate_limits": map[string]any{
			"five_hour": map[string]any{"used_percentage": 62.5, "resets_at": now + 2*3600 + 12*60},
			"seven_day": map[string]any{"used_percentage": 85.2, "resets_at": now + 3*86400 + 4*3600},
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

// hit returns the id of the zone under the mouse ("" if none).
func (m model) hit(msg tea.MouseMsg) string {
	ids := []string{"tab:0", "tab:1", "tab:2", "btn:install", "palette"}
	for r, row := range m.cfg.Rows {
		ids = append(ids, fmt.Sprintf("row:%d", r), fmt.Sprintf("gap:%d", r), fmt.Sprintf("free:%d", r))
		for c := range row {
			ids = append(ids, cellID(r, c), sepSlotID(r, c))
		}
	}
	for _, w := range widgets {
		ids = append(ids, "pal:"+w.ID)
	}
	for _, sec := range []string{"Color", "Bar", "Display", "Format", "Text", "Width", "Align"} {
		ids = append(ids, "sec:"+sec)
	}
	ids = append(ids, optIDs()...)
	for _, sp := range separators {
		ids = append(ids, "sep:"+sp, "cellsep:"+sp)
	}
	ids = append(ids, "cellsep:")
	for _, t := range themes {
		ids = append(ids, "theme:"+t.ID)
	}
	for _, p := range presets {
		ids = append(ids, "preset:"+p.ID)
	}
	ids = append(ids, "preset:random", "preset:prev", "ui:", "ui:ocean", "ui:paper")
	for _, b := range borders {
		ids = append(ids, "border:"+b.ID)
	}
	for _, c := range colors {
		ids = append(ids, "bcolor:"+c.Name)
	}
	ids = append(ids, "border:", "bcolor:")
	ids = append(ids, "btn:+row", "btn:-row", "btn:+col", "btn:-col", "btn:airrows", "btn:aircols")
	// Nested zones (pal:… inside palette) are checked before their container.
	for _, id := range ids[5:] {
		if zone.Get(id).InBounds(msg) {
			return id
		}
	}
	for _, id := range ids[:5] {
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
		m.plan, m.oldCmd, m.newCmd, m.planErr = planInstall()
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

func (m *model) install() tea.Cmd {
	if m.planErr != nil {
		return m.notify("settings.json unreadable: "+m.planErr.Error(), true)
	}
	if cmd := m.save(); m.dirty() {
		return cmd // saving the config failed
	}
	if err := applyInstall(m.plan); err != nil {
		return m.notify("Cannot install: "+err.Error(), true)
	}
	m.setTab(2) // re-read settings.json to refresh the status
	return m.notify("Installed in Claude Code", false)
}

func (m model) press(hit string) (tea.Model, tea.Cmd) {
	m.editing = false
	if p, ok := parseCell(hit); ok {
		m.selRow, m.selSep = nil, nil
		if c := m.cfg.Rows[p.r][p.c]; c.Widget != "" {
			m.drag = &drag{from: &p, cell: c}
		} else {
			m.sel = &p
		}
		return m, nil
	}
	switch {
	case strings.HasPrefix(hit, "row:"):
		r, _ := strconv.Atoi(strings.TrimPrefix(hit, "row:"))
		m.sel, m.selRow, m.selSep = nil, &r, nil
	case strings.HasPrefix(hit, "sepslot:"):
		var p pos
		fmt.Sscanf(hit, "sepslot:%d:%d", &p.r, &p.c)
		m.sel, m.selRow, m.selSep = nil, nil, &p
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
	case hit == "btn:install":
		return m, m.install()
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
		m.roll()
		return m, m.notify("Random · r reroll · R previous · s save", false)
	case hit == "preset:prev":
		if m.prevRoll() {
			return m, m.notify(fmt.Sprintf("Roll %d of %d · s save", m.rollIdx+1, len(m.rolls)), false)
		}
	case strings.HasPrefix(hit, "preset:"):
		for _, p := range presets {
			if "preset:"+p.ID == hit {
				m.cfg, m.sel, m.selRow, m.selSep = p.build(), nil, nil, nil
				return m, m.notify(p.Label+" applied · s to save", false)
			}
		}
	case strings.HasPrefix(hit, "ui:"):
		m.cfg.UI = strings.TrimPrefix(hit, "ui:")
		applyUI(m.cfg.UI)
	case strings.HasPrefix(hit, "theme:"):
		m.applyTheme(strings.TrimPrefix(hit, "theme:"))
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
	if m.selRow != nil && *m.selRow >= len(m.cfg.Rows) {
		m.selRow = nil
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
	case strings.HasPrefix(id, "place:"):
		*c = Cell{Widget: strings.TrimPrefix(id, "place:")}
		if th, ok := themeByID(m.cfg.Theme); ok {
			c.Color = th.color(c.Widget)
		}
		m.editing = c.Widget == "text"
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
		m.sel, m.selRow, m.selSep = &pos{0, 0}, nil, nil
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
		m.sel, m.selRow, m.selSep = nil, nil, nil
	case "pgdown":
		m.scrollBy(m.bodyRoom() - 2)
	case "pgup":
		m.scrollBy(-(m.bodyRoom() - 2))
	case "r":
		m.roll()
		return m, m.notify("Random · r reroll · R previous · s save", false)
	case "R":
		if m.prevRoll() {
			return m, m.notify(fmt.Sprintf("Roll %d of %d · s save", m.rollIdx+1, len(m.rolls)), false)
		}
	case "s":
		return m, m.save()
	case "i":
		m.setTab(2)
	case "enter":
		if m.tab == 2 {
			return m, m.install()
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
	return zone.Mark(id, toggleStyle(on).Render(label))
}

func themeByID(id string) (theme, bool) {
	for _, t := range themes {
		if t.ID == id {
			return t, true
		}
	}
	return theme{}, false
}

// applyTheme recolors all cells and the border (overrides hand-picked colors).
func (m *model) applyTheme(id string) {
	t, ok := themeByID(id)
	if !ok {
		return
	}
	m.cfg.Theme, m.cfg.BorderColor = id, t.Border
	for r := range m.cfg.Rows {
		for c := range m.cfg.Rows[r] {
			if w := m.cfg.Rows[r][c].Widget; w != "" {
				m.cfg.Rows[r][c].Color = t.color(w)
			}
		}
	}
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
	left := " " + title.Render("slb") + "   "
	for i, n := range tabNames {
		st := faint
		if i == m.tab {
			st = lipgloss.NewStyle().Bold(true).Underline(true).Foreground(accent)
		}
		left += zone.Mark(fmt.Sprintf("tab:%d", i), st.Render(n)) + "   "
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
	pcfg := m.cfg // same global settings, rows copied below
	pcfg.Rows = nil
	for _, row := range m.cfg.Rows {
		prow := append([]Cell(nil), row...)
		for i, c := range prow {
			if c.Widget == "git" {
				prow[i].Widget, prow[i].Text = "text", m.branch // keeps color, alignment and separator
			}
		}
		pcfg.Rows = append(pcfg.Rows, prow)
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
		return hints("drag", "place", "click", "edit", "1-3", "tab", "s", "save", "q", "quit")
	case m.tab == 1:
		return hints("click", "choose", "1-3", "tab", "s", "save", "q", "quit")
	}
	return hints("enter", "save and install", "1-3", "tab", "q", "quit")
}

// labelW: width of the label column, shared by all settings.
const labelW = 14

// field lays out a setting: label in a fixed column, controls on the right.
func field(name, content string) string {
	return lipgloss.JoinHorizontal(lipgloss.Top, faint.Width(labelW).Render(name), content)
}

// swatch: one square of the color grid.
type swatch struct {
	ID     string   // stored value ("" = default color)
	Colors []string // colors shown side by side; empty = text chip (Label)
	Label  string
}

// colorItems: the 24 colors, then the "default" choice (last, so that
// keyboard navigation lines up with the rows of 8).
func colorItems(defaultLabel string) []swatch {
	var items []swatch
	for _, c := range colors {
		items = append(items, swatch{ID: c.Name, Colors: []string{c.Name}})
	}
	return append(items, swatch{ID: "", Label: defaultLabel})
}

// swatchGrid draws clickable color squares (zone prefix+ID), perRow per row,
// with a blank line between rows. A square is 2 characters on 1 line, i.e. square
// on screen. Colorless choices ("auto") are buttons at the end of the first row.
func swatchGrid(prefix, cur, focus string, items []swatch, perRow int) string {
	var squares, chips []string
	for _, it := range items {
		var cell string
		switch {
		case len(it.Colors) == 0:
			cell = toggleStyle(it.ID == cur).Underline(it.ID == focus && focus != "").Render(it.Label)
			chips = append(chips, zone.Mark(prefix+it.ID, cell))
			continue
		case it.ID == focus && focus != "":
			cell = swatchCell(it, "◆")
		case it.ID == cur:
			cell = swatchCell(it, "✓")
		default:
			cell = swatchCell(it, "")
		}
		squares = append(squares, zone.Mark(prefix+it.ID, cell))
	}
	var rows []string
	for i := 0; i < len(squares); i += perRow {
		row := strings.Join(squares[i:min(i+perRow, len(squares))], " ")
		if i == 0 && len(chips) > 0 {
			row += "   " + strings.Join(chips, " ")
		}
		rows = append(rows, row)
	}
	return strings.Join(rows, "\n\n")
}

// swatchCell: 2 background characters per color; mark (✓ chosen, ◆ keyboard focus) on the middle color.
func swatchCell(s swatch, mark string) string {
	var b strings.Builder
	for i, c := range s.Colors {
		txt := "  "
		if mark != "" && i == len(s.Colors)/2 {
			txt = mark + " "
		}
		fg := lipgloss.Color("0")
		if !isLight(colorSpec(c)) {
			fg = lipgloss.Color("15")
		}
		b.WriteString(lipgloss.NewStyle().Background(lipgloss.Color(colorSpec(c))).Foreground(fg).Bold(true).Render(txt))
	}
	return b.String()
}

// isLight tells whether a color (256 index or #rrggbb) is light, to pick
// a readable black or white check mark on it. The 16 base colors depend on the
// terminal theme: default xterm values.
func isLight(spec string) bool {
	var r, g, b int
	if strings.HasPrefix(spec, "#") {
		fmt.Sscanf(spec, "#%02x%02x%02x", &r, &g, &b)
	} else {
		n, _ := strconv.Atoi(spec)
		switch {
		case n < 16:
			return n == 2 || n == 3 || n == 6 || n == 7 || n >= 10 && n != 12
		case n < 232:
			lv := []int{0, 95, 135, 175, 215, 255}
			n -= 16
			r, g, b = lv[n/36], lv[n/6%6], lv[n%6]
		default:
			r = 8 + (n-232)*10
			g, b = r, r
		}
	}
	return 299*r+587*g+114*b > 140000
}

func (m model) styleTab() string {
	// Themes: 3 per row, fixed-width names so the columns line up.
	var themeRows []string
	for i := 0; i < len(themes); i += 3 {
		var chips []string
		for _, t := range themes[i:min(i+3, len(themes))] {
			sw := ""
			for _, c := range []string{t.Model, t.Dir, t.Git, t.Gauge[2]} {
				sw += lipgloss.NewStyle().Foreground(lipgloss.Color(colorSpec(c))).Render("█")
			}
			chips = append(chips, toggle("theme:"+t.ID, fmt.Sprintf("%-11s", t.Label), m.cfg.Theme == t.ID)+" "+sw)
		}
		themeRows = append(themeRows, strings.Join(chips, "   "))
	}

	var seps []string
	for _, sp := range separators {
		label := sp
		if sp == "space" {
			label = "space"
		}
		seps = append(seps, toggle("sep:"+sp, label, m.cfg.Sep == sp || m.cfg.Sep == "" && sp == "│"))
	}

	borderChips := []string{toggle("border:", "none", m.cfg.Border == "")}
	for _, b := range borders {
		borderChips = append(borderChips, toggle("border:"+b.ID, b.TL+b.H+b.TR, m.cfg.Border == b.ID))
	}

	auto := "auto (dark)"
	if !darkTerminal {
		auto = "auto (light)"
	}
	app := chips(opt{"ui:", auto, m.cfg.UI == ""}, opt{"ui:ocean", "Ocean", m.cfg.UI == "ocean"}, opt{"ui:paper", "Paper", m.cfg.UI == "paper"}) +
		faint.Render("   colours of slb itself")
	parts := []string{
		field("App", app), "",
		field("Theme", strings.Join(themeRows, "\n\n")), "",
		field("Separator", strings.Join(seps, " ")), "",
		field("Border", strings.Join(borderChips, " ")),
	}
	if m.cfg.Border != "" {
		parts = append(parts, "", field("", swatchGrid("bcolor:", m.cfg.BorderColor, "", colorItems("gray"), 8)))
	}
	parts = append(parts, "", field("Spacing",
		toggle("btn:airrows", "blank line between rows", m.allGaps())+" "+
			toggle("btn:aircols", "spaced columns", m.cfg.AirCols)))
	return strings.Join(parts, "\n")
}

func (m model) installTab() string {
	if m.planErr != nil {
		return m.titled(heading.Render("INSTALLATION")) + "\n\n" +
			errStyle.Render("✗ ") + "Cannot read ~/.claude/settings.json: " + m.planErr.Error() + "\n" +
			faint.Render("Fix the file, then reopen this tab.")
	}
	status := warnStyle.Render("○ ") + "slb is not yet the Claude Code status line"
	if m.oldCmd == m.newCmd {
		status = okStyle.Render("✓ ") + "slb is the Claude Code status line"
	}
	cfgState := faint.Render("  ✓ saved")
	if m.dirty() {
		cfgState = warnStyle.Render("  ● unsaved")
	}
	lines := []string{
		m.titled(heading.Render("INSTALLATION")), "",
		faint.Render("Status      ") + status,
		faint.Render("Config      ") + "~/.claude/statusline-builder.json" + cfgState,
		"",
	}
	if m.oldCmd != m.newCmd {
		lines = append(lines,
			faint.Render("Planned change in ~/.claude/settings.json"),
			"  "+errStyle.Render("- "+m.oldCmd),
			"  "+okStyle.Render("+ "+m.newCmd),
			"",
		)
	}
	lines = append(lines,
		zone.Mark("btn:install", button.Background(accent).Foreground(onAccent).Bold(true).Render("Save and install")),
		"",
		faint.Render("A copy of settings.json is kept in settings.json.bak before each change."),
	)
	return strings.Join(lines, "\n")
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
	h := m.h
	if h == 0 {
		h = 24
	}
	return max(3, h-3-lipgloss.Height(m.previewBox())-1)
}

func (m *model) scrollBy(d int) {
	m.scroll = min(max(0, m.scroll+d), max(0, len(m.bodyLines())-m.bodyRoom()))
}

func (m model) View() string {
	if m.w > 0 && (m.w < minW || m.h < minH) {
		return fmt.Sprintf("\n  Terminal too small: %d×%d minimum (currently %d×%d).\n  Enlarge the window, or press %s to quit.",
			minW, minH, m.w, m.h, keyStyle.Render("q"))
	}

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

	if m.drag != nil {
		view = overlay(view, dragLabel.Render("⠿ "+label(m.drag.cell.Widget)), m.mx+2, m.my)
	}
	return view
}

// overlay draws fg on line y of bg starting at column x (label following the cursor).
func overlay(bg, fg string, x, y int) string {
	lines := strings.Split(bg, "\n")
	if y < 0 || y >= len(lines) {
		return bg
	}
	l := lines[y]
	if w := ansi.StringWidth(l); w < x {
		l += strings.Repeat(" ", x-w)
	}
	lines[y] = ansi.Truncate(l, x, "") + "\x1b[0m" + fg + ansi.TruncateLeft(l, x+ansi.StringWidth(fg), "")
	return strings.Join(lines, "\n")
}
