package main

// Widget catalogue: a window over the builder listing every widget, to choose
// which ones the Layout tab offers. Type to filter, space to add or remove.

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// groupNames: widget groups, in the order of their first widget.
func groupNames() []string {
	var names []string
	for _, w := range widgets {
		if !slices.Contains(names, w.Group) {
			names = append(names, w.Group)
		}
	}
	return names
}

func (m model) onGrid(id string) bool {
	for _, row := range m.cfg.Rows {
		for _, c := range row {
			if c.Widget == id {
				return true
			}
		}
	}
	return false
}

// inPalette: offered in the Layout tab, because it was chosen or is on the grid.
func (m model) inPalette(w widgetInfo) bool {
	return slices.Contains(m.cfg.Palette, w.ID) || m.onGrid(w.ID)
}

// toggleWidget adds a widget to the Layout tab list, or takes it out.
func (m *model) toggleWidget(id string) tea.Cmd {
	if m.onGrid(id) {
		return m.notify(label(id)+" is on the grid: remove it from there first", false)
	}
	if i := slices.Index(m.cfg.Palette, id); i >= 0 {
		m.cfg.Palette = slices.Delete(m.cfg.Palette, i, i+1)
	} else {
		m.cfg.Palette = append(m.cfg.Palette, id)
	}
	return nil
}

// catLine: one line of the catalogue list, a group heading (id "") or a widget.
type catLine struct{ id, text string }

// catTop: screen line of the window's top edge, just under the header.
const catTop = 1

// catWidgets: the line index of each widget in lines, in order.
func catWidgets(lines []catLine) []int {
	var at []int
	for i, l := range lines {
		if l.id != "" {
			at = append(at, i)
		}
	}
	return at
}

// catItems: the widgets matching the filter, under their group headings.
func (m model) catItems() []catLine {
	q := strings.ToLower(m.catQuery)
	var s session
	_ = json.Unmarshal(m.sample, &s)
	th, _ := themeByID(m.cfg.Theme)
	var lines []catLine
	for _, g := range groupNames() {
		head := false
		for _, w := range widgets {
			if w.Group != g || !strings.Contains(strings.ToLower(w.Label+" "+w.Desc+" "+w.Group), q) {
				continue
			}
			if !head {
				lines, head = append(lines, catLine{"", heading.Render(strings.ToUpper(g))}), true
			}
			box := faint.Render("[ ]")
			if m.inPalette(w) {
				box = okStyle.Render("[✓]")
			}
			var val string
			switch w.ID {
			case "git":
				val = paint(m.branch, colorSpec(th.color("git")))
			case "text", "spacer":
				val = faint.Render("—")
			default:
				val = cellValue(Cell{Widget: w.ID, Color: th.color(w.ID)}, s)
			}
			val = ansi.Truncate(val, 20, "…")
			lines = append(lines, catLine{w.ID, fmt.Sprintf("%s %-14s %s%s %s", box, w.Label, val,
				strings.Repeat(" ", max(0, 20-ansi.StringWidth(val))), faint.Render(w.Desc))})
		}
	}
	return lines
}

// catRoom: list lines shown at once in the window.
func (m model) catRoom() int {
	return max(3, m.height()-10)
}

// catBox draws the window, and returns it with its column on screen.
func (m model) catBox() (box string, x int) {
	lines := m.catItems()
	cur := m.catID()
	search := faint.Render("type to filter…")
	if m.catQuery != "" {
		search = m.catQuery + lipgloss.NewStyle().Foreground(accent).Render("▏")
	}
	room := m.catRoom()
	shown := lines[min(m.catScroll, len(lines)):min(len(lines), m.catScroll+room)]
	body := []string{"⌕ " + search, ""}
	for _, l := range shown {
		t := l.text
		if l.id != "" && l.id == cur {
			t = selStyle.Render("›") + t
		} else {
			t = " " + t
		}
		body = append(body, t)
	}
	if len(lines) == 0 {
		body = append(body, faint.Render(" No widget matches."))
	}
	for len(body) < room+2 {
		body = append(body, "")
	}
	n := 0
	for _, w := range widgets {
		if m.inPalette(w) {
			n++
		}
	}
	body = append(body, "", hints("↑↓", "move", "space", "add or remove", "esc", "close"))
	w := min(84, m.width()-2)
	box = titledBox(fmt.Sprintf("Widgets · %d of %d in the list", n, len(widgets)), strings.Join(body, "\n"), w)
	return box, (m.width() - w) / 2
}

// catAt: the index of the list line on screen line y, -1 if none (n lines listed).
func (m model) catAt(y, n int) int {
	i := y - catTop - 3 // border, search line, blank line
	if i < 0 || i >= m.catRoom() || i+m.catScroll >= n {
		return -1
	}
	return i + m.catScroll
}

// catMove moves the selection by d widgets and keeps it in view.
func (m *model) catMove(d int) {
	at := catWidgets(m.catItems())
	if len(at) == 0 {
		m.catSel, m.catScroll = 0, 0
		return
	}
	m.catSel = min(max(m.catSel+d, 0), len(at)-1)
	line := at[m.catSel]
	if m.catSel == 0 {
		line = 0 // show the first group heading
	}
	room := m.catRoom()
	m.catScroll = min(max(m.catScroll, line-room+1), line)
}

// catID: the selected widget, "" if the filter matches nothing.
func (m model) catID() string {
	lines := m.catItems()
	if at := catWidgets(lines); m.catSel < len(at) {
		return lines[at[m.catSel]].id
	}
	return ""
}

func (m model) catKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		if m.catQuery != "" {
			m.catQuery = ""
		} else {
			m.cat = false
		}
	case tea.KeyUp:
		m.catMove(-1)
	case tea.KeyDown:
		m.catMove(1)
	case tea.KeyPgUp:
		m.catMove(-m.catRoom())
	case tea.KeyPgDown:
		m.catMove(m.catRoom())
	case tea.KeySpace, tea.KeyEnter:
		if id := m.catID(); id != "" {
			return m, m.toggleWidget(id)
		}
	case tea.KeyBackspace:
		if r := []rune(m.catQuery); len(r) > 0 {
			m.catQuery = string(r[:len(r)-1])
			m.catSel, m.catScroll = 0, 0
		}
	case tea.KeyRunes:
		m.catQuery += string(msg.Runes)
		m.catSel, m.catScroll = 0, 0
	}
	return m, nil
}

func (m model) catMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Button == tea.MouseButtonWheelUp && msg.Action == tea.MouseActionPress:
		m.catMove(-3)
	case msg.Button == tea.MouseButtonWheelDown && msg.Action == tea.MouseActionPress:
		m.catMove(3)
	case msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress:
		box, x := m.catBox()
		if msg.X < x || msg.X >= x+lipgloss.Width(box) || msg.Y < catTop || msg.Y >= catTop+lipgloss.Height(box) {
			m.cat = false // click outside the window
			return m, nil
		}
		lines := m.catItems()
		if i := m.catAt(msg.Y, len(lines)); i >= 0 && lines[i].id != "" {
			m.catSel = slices.Index(catWidgets(lines), i)
			return m, m.toggleWidget(lines[i].id)
		}
	}
	return m, nil
}
