package main

// Layout tab: grid of cards, widgets as chips, then the settings of the selected
// cell (or row), grouped in sections. Every choice is visible.

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone"
)

var widgetGroups = []struct {
	Name string
	IDs  []string
}{
	{"Session", []string{"model", "dir", "git", "userhost"}},
	{"Gauges", []string{"context", "limit5h", "limit7d"}},
	{"Activity", []string{"cost", "lines", "reset5h", "reset7d", "text"}},
	{"Layout", []string{"spacer"}},
}

// optIDs: all clickable choices in the sections (fixed zones).
func optIDs() []string {
	ids := []string{"opt:align:", "opt:align:center", "opt:align:right", "opt:mode:", "opt:mode:left", "opt:width:1", "opt:width:2", "opt:width:4",
		"opt:width:5", "opt:width:8", "opt:width:10", "opt:width:15", "opt:width:16",
		"opt:label:on", "opt:label:off", "opt:icon:yes", "opt:icon:no",
		"opt:version:yes", "opt:version:no", "opt:full:yes", "opt:full:no", "insp:text", "color:"}
	for _, b := range barStyles {
		ids = append(ids, "opt:bar:"+b.ID)
	}
	for _, cp := range capStyles {
		ids = append(ids, "opt:caps:"+cp.ID)
	}
	for _, c := range colors {
		ids = append(ids, "color:"+c.Name)
	}
	for name := range schemes {
		ids = append(ids, "color:"+name)
	}
	return ids
}

func (m model) selCell() *Cell {
	if m.sel == nil {
		return nil
	}
	return &m.cfg.Rows[m.sel.r][m.sel.c]
}

// gapChipW: room for the "blank line" button at the end of each grid row.
const gapChipW = 27

// cardW: card width, shrunk so the grid and its buttons fit in the terminal.
func (m model) cardW() int {
	n := len(m.cfg.Rows[0])
	return min(17, max(10, (m.width()-2-4-(n-1)*3-gapChipW)/n))
}

func (m model) layoutTab() string {
	btns := lipgloss.JoinHorizontal(lipgloss.Top,
		toggle("btn:+row", "+ row", false), " ",
		toggle("btn:-row", "− row", false), "   ",
		toggle("btn:+col", "+ column", false), " ",
		toggle("btn:-col", "− column", false))
	// Random comes first, in gold: the quickest way to a starting point.
	random := lipgloss.NewStyle().Padding(0, 1).Bold(true).Background(randomBg).Foreground(lipgloss.Color("0"))
	if m.drag == nil && m.hover == "preset:random" {
		random = random.Background(randomHover)
	}
	roll := []string{zone.Mark("preset:random", random.Render("⚄ Random"))}
	if m.rollIdx > 0 {
		roll = append(roll, toggle("preset:prev", "↶ Previous", false))
	}
	roll = append(roll, faint.Render(" r to reroll"))
	var ps []string
	for _, p := range presets {
		ps = append(ps, toggle("preset:"+p.ID, p.Label, false))
	}
	parts := []string{
		m.titled(heading.Render("PRESETS")), faint.Render("Start from a ready-made layout. It replaces the current grid and style."), "",
		strings.Join(roll, " "), "", strings.Join(ps, " "), "", "",
		m.titled(heading.Render("GRID")), "", m.cardsView(), "", "    " + btns, "", "", m.widgetsView()}
	if s := m.settingsView(); s != "" {
		parts = append(parts, "", "", s)
	}
	return strings.Join(parts, "\n")
}

func (m model) cardsView() string {
	var rows []string
	for r, row := range m.cfg.Rows {
		g := fmt.Sprintf("L%d", r+1)
		gst := faint
		if m.selRow != nil && *m.selRow == r {
			gst = selStyle
		}
		cards := []string{zone.Mark(fmt.Sprintf("row:%d", r), gst.Render(g)) + strings.Repeat(" ", 4-len(g))}
		for c := range row {
			if c > 0 {
				cards = append(cards, m.sepSlot(r, c-1))
			}
			cards = append(cards, m.card(r, c))
		}
		cards = append(cards, "  ", toggle(fmt.Sprintf("gap:%d", r), "↵ blank line", m.gap(r)),
			" ", toggle(fmt.Sprintf("free:%d", r), "off grid", m.cfg.free(r)))
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Center, cards...))
		if m.gap(r) {
			w := len(row)*(m.cardW()+1) - 1
			rows = append(rows, "    "+faint.Render(lipgloss.PlaceHorizontal(w, lipgloss.Center, " blank line ", lipgloss.WithWhitespaceChars("┄"))))
		}
	}
	return strings.Join(rows, "\n")
}

// sepSlot is the clickable gap after cell (r, c): it shows the separator rendered there,
// grey when inherited from the Style tab, pink when picked for this spot.
func (m model) sepSlot(r, c int) string {
	id := m.cfg.Rows[r][c].Sep
	glyph, st := m.cfg.Sep, faint
	if id != "" {
		glyph, st = id, lipgloss.NewStyle().Foreground(accent).Bold(true)
	}
	switch glyph {
	case "":
		glyph = "│"
	case "space":
		glyph = "␣"
	}
	if m.selSep != nil && *m.selSep == (pos{r, c}) {
		st = selStyle
	} else if m.drag == nil && m.hover == sepSlotID(r, c) {
		st = st.Foreground(accent)
	}
	return zone.Mark(sepSlotID(r, c), st.Render(" "+glyph+" "))
}

func sepSlotID(r, c int) string { return fmt.Sprintf("sepslot:%d:%d", r, c) }

func (m model) card(r, c int) string {
	cell := m.cfg.Rows[r][c]
	w := m.cardW()
	text := faint.Render("·")
	if cell.Widget != "" {
		t := label(cell.Widget)
		if cell.Widget == "text" && cell.Text != "" {
			t = "« " + cell.Text + " »"
		}
		if m.editing && m.sel != nil && *m.sel == (pos{r, c}) {
			t = "« " + cell.Text + "▏»"
		}
		text = colorOf(cell.Color).Render(ansi.Truncate(t, w-2, "…"))
		if cell.Widget == "spacer" {
			text = faint.Render(ansi.Truncate(fmt.Sprintf("␣ spacer %d", spacerWidth(cell)), w-2, "…"))
		}
	}
	// The card text follows the cell alignment, so the grid shows where the value will sit.
	place := map[string]lipgloss.Position{"": lipgloss.Left, "center": lipgloss.Center, "right": lipgloss.Right}[cell.Align]
	if cell.Widget == "" || cell.Widget == "spacer" {
		place = lipgloss.Center
	}
	st := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(edgeColor).Width(w-2).Align(place).Padding(0, 1)
	if m.drag == nil && m.hover == cellID(r, c) {
		st = st.BorderForeground(accent) // hover
	}
	switch id := cellID(r, c); {
	case m.drag != nil && m.hover == id:
		st = st.Border(lipgloss.ThickBorder()).BorderForeground(accent)
	case m.drag != nil && m.drag.from != nil && *m.drag.from == (pos{r, c}):
		text = faint.Render(ansi.Strip(text))
	case m.sel != nil && *m.sel == (pos{r, c}):
		st = st.Border(lipgloss.DoubleBorder()).BorderForeground(accent)
		text = lipgloss.NewStyle().Foreground(accent).Bold(true).Render(ansi.Strip(text))
	}
	return zone.Mark(cellID(r, c), st.Render(text))
}

func (m model) widgetsView() string {
	head := heading.Render("WIDGETS")
	if m.drag != nil && m.drag.from != nil && (m.hover == "palette" || strings.HasPrefix(m.hover, "pal:")) {
		head = errStyle.Bold(true).Render("RELEASE TO REMOVE THE WIDGET")
	}
	head = m.titled(head) + "\n" + faint.Render("Drag a widget onto the grid, or drag a cell back here to remove it.")
	var groups []string
	for _, g := range widgetGroups {
		var chips []string
		for i, id := range g.IDs {
			if i > 0 {
				chips = append(chips, " ")
			}
			edge := edgeColor
			if m.drag == nil && m.hover == "pal:"+id || m.drag != nil && m.drag.from == nil && m.drag.cell.Widget == id {
				edge = accent // hovered, or being dragged
			}
			chips = append(chips, zone.Mark("pal:"+id, lipgloss.NewStyle().Border(dashedBorder).BorderForeground(edge).Padding(0, 1).Render(label(id))))
		}
		// Group name on the middle line of the buttons, in a narrow column of its own.
		groups = append(groups, lipgloss.JoinHorizontal(lipgloss.Center, groupStyle.Render(g.Name), lipgloss.JoinHorizontal(lipgloss.Top, chips...)))
	}
	// Pack the groups: each one goes on the first line with room left (first fit), so
	// small groups share a line instead of taking one each.
	room := m.width() - 4
	var lines [][]string
	var used []int
	for _, g := range groups {
		w := lipgloss.Width(g)
		placed := false
		for i := range lines {
			if used[i]+3+w <= room {
				lines[i], used[i], placed = append(lines[i], "   ", g), used[i]+3+w, true
				break
			}
		}
		if !placed {
			lines, used = append(lines, []string{g}), append(used, w)
		}
	}
	var rows []string
	for _, l := range lines {
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, l...))
	}
	// No gap between lines: button borders already separate them.
	return zone.Mark("palette", head+"\n\n"+strings.Join(rows, "\n"))
}

// dashedBorder: dashed border for the widget list buttons ("draggable").
var dashedBorder = lipgloss.Border{
	Top: "╌", Bottom: "╌", Left: "╎", Right: "╎",
	TopLeft: "╭", TopRight: "╮", BottomLeft: "╰", BottomRight: "╯",
}

// sections: the settings sections for the cell's widget.
func sections(c *Cell) []string {
	switch {
	case isGauge(c.Widget):
		return []string{"Color", "Bar", "Display", "Align"}
	case c.Widget == "model", c.Widget == "dir":
		return []string{"Color", "Format", "Align"}
	case c.Widget == "text":
		return []string{"Color", "Text", "Align"}
	case c.Widget == "spacer":
		return []string{"Width"}
	}
	return []string{"Color", "Align"}
}

// curSection: the open section, or the first one if the chosen one doesn't exist for this widget.
func (m model) curSection() string {
	c := m.selCell()
	if c == nil || c.Widget == "" {
		return ""
	}
	secs := sections(c)
	for _, s := range secs {
		if s == m.section {
			return s
		}
	}
	return secs[0]
}

type opt struct {
	ID, Label string
	On        bool
}

func chips(opts ...opt) string {
	var out []string
	for _, o := range opts {
		out = append(out, toggle(o.ID, o.Label, o.On))
	}
	return strings.Join(out, " ")
}

func (m model) settingsView() string {
	if p := m.selSep; p != nil {
		cur := m.cfg.Rows[p.r][p.c].Sep
		inherited := m.cfg.Sep
		if inherited == "" {
			inherited = "│"
		}
		var o []opt
		o = append(o, opt{"cellsep:", "inherit " + inherited, cur == ""})
		for _, sp := range separators {
			name := sp
			if sp == "space" {
				name = "space"
			}
			o = append(o, opt{"cellsep:" + sp, name, cur == sp})
		}
		return m.titled(heading.Render("SEPARATOR")+faint.Render(fmt.Sprintf("  row %d · after column %d", p.r+1, p.c+1))) + "\n\n" +
			chips(o...) + "\n\n" + faint.Render("inherit follows the separator chosen in the Style tab.")
	}
	if m.selRow != nil {
		r := *m.selRow
		return m.titled(heading.Render(fmt.Sprintf("ROW %d", r+1))) + "\n\n" +
			field("Blank line", toggle(fmt.Sprintf("gap:%d", r), "after this row", m.gap(r)))
	}
	c := m.selCell()
	if c == nil {
		return ""
	}
	where := faint.Render(fmt.Sprintf("  row %d · column %d", m.sel.r+1, m.sel.c+1))
	if c.Widget == "" {
		return m.titled(heading.Render("EMPTY CELL")+where) + "\n\n" + faint.Render("Drag a widget from the list above.")
	}

	cur := m.curSection()
	var tabs []string
	for _, s := range sections(c) {
		st := faint
		if s == cur {
			st = lipgloss.NewStyle().Bold(true).Underline(true).Foreground(accent)
		}
		tabs = append(tabs, zone.Mark("sec:"+s, st.Render(s)))
	}

	var body []string
	switch cur {
	case "Color":
		// The section name is enough: swatches align left, without a label.
		body = append(body, swatchGrid("color:", c.Color, "", colorItems("auto"), 8))
		if isGauge(c.Widget) {
			var sw []swatch
			for _, name := range schemeOrder {
				sc := schemes[name]
				sw = append(sw, swatch{ID: name, Colors: sc[:]})
			}
			body = append(body, faint.Render("Thresholds")+"\n\n"+swatchGrid("color:", c.Color, "", sw, 4))
		}
	case "Bar":
		var rows, row []string
		for i, b := range barStyles {
			fill, empty := bar(b.ID, 42, 4)
			sample := fill + empty
			if b.ID == "text" {
				sample = "42%"
			}
			row = append(row, toggle("opt:bar:"+b.ID, sample+" "+b.Label, barLabel(c.Bar) == b.Label))
			if len(row) == 5 || i == len(barStyles)-1 {
				rows = append(rows, strings.Join(row, " "))
				row = nil
			}
		}
		body = append(body, strings.Join(rows, "\n\n"))
	case "Display":
		w := c.Width
		if w == 0 {
			w = 10
		}
		var caps []opt
		for _, cp := range capStyles {
			name := cp.Left + " " + cp.Right
			if cp.ID == "none" {
				name = "none"
			}
			caps = append(caps, opt{"opt:caps:" + cp.ID, name, capsOf(*c) == cp.ID})
		}
		body = append(body,
			field("Show", chips(opt{"opt:mode:", "used", c.Mode == ""}, opt{"opt:mode:left", "left", c.Mode == "left"})),
			field("Width", chips(opt{"opt:width:5", "5", w == 5}, opt{"opt:width:10", "10", w == 10}, opt{"opt:width:15", "15", w == 15})),
			field("Caps", chips(caps...)),
			field("Label", chips(opt{"opt:label:on", "yes", !c.NoLabel}, opt{"opt:label:off", "no", c.NoLabel})))
	case "Format":
		if c.Widget == "model" {
			body = append(body,
				field("Format", chips(opt{"opt:icon:no", "Opus", !c.Icon}, opt{"opt:icon:yes", "✳ Opus", c.Icon})),
				field("Version", chips(opt{"opt:version:no", "no", !c.Version}, opt{"opt:version:yes", "yes", c.Version})))
		} else {
			body = append(body, chips(opt{"opt:full:no", "folder name", !c.Full}, opt{"opt:full:yes", "~ path", c.Full}))
		}
	case "Width":
		w := spacerWidth(*c)
		var ws []opt
		for _, n := range []int{1, 2, 4, 8, 16} {
			ws = append(ws, opt{fmt.Sprintf("opt:width:%d", n), fmt.Sprint(n), w == n})
		}
		body = append(body, chips(ws...)+faint.Render("  spaces reserved in the column"))
	case "Align":
		body = append(body, chips(
			opt{"opt:align:", "◧ left", c.Align == ""},
			opt{"opt:align:center", "◫ center", c.Align == "center"},
			opt{"opt:align:right", "◨ right", c.Align == "right"})+
			"\n\n"+faint.Render("Where the value sits when its column is wider than it."))
	case "Text":
		body = append(body, "« "+c.Text+" »   "+toggle("insp:text", "edit", m.editing))
	}

	return m.titled(heading.Render(strings.ToUpper(label(c.Widget)))+where) + "\n\n" +
		strings.Join(tabs, "    ") + "\n\n" + strings.Join(body, "\n\n")
}

// titled extends a block title with a gray rule up to the right edge.
func (m model) titled(title string) string {
	rule := m.width() - 6 - ansi.StringWidth(title) // indent of 2, right margin of 2, spaces
	if rule < 2 {
		return title
	}
	return title + " " + lipgloss.NewStyle().Foreground(ruleColor).Render(strings.Repeat("─", rule))
}

func barLabel(id string) string {
	if id == "" {
		id = "blocks"
	}
	for _, b := range barStyles {
		if b.ID == id {
			return b.Label
		}
	}
	return id
}
