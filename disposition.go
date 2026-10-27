package main

// Onglet Disposition : panneau des widgets, grille en tableau, inspecteur de la
// cellule ou de la ligne sélectionnée, et listes de choix ouvertes par-dessus.

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone"
)

const (
	panelW = 18 // panneau des widgets
	inspW  = 28 // inspecteur
)

// colW : largeur d'une cellule de la grille, réduite (jusqu'à 9) pour tenir à côté du panneau.
func (m model) colW() int {
	insp := m.sel != nil || m.selRow != nil
	side := 0 // étroit : grille seule
	switch {
	case m.width() >= 100 && insp:
		side = panelW + 2 + inspW + 3
	case m.width() >= 100 || m.width() >= 80 && !insp:
		side = panelW
	case m.width() >= 80:
		side = inspW
	}
	room := m.width() - 2 - side - 2 - 5 // retrait, panneau, espace, gouttière et bord gauche
	return min(12, max(9, room/len(m.cfg.Rows[0])-1))
}

var (
	selStyle  = lipgloss.NewStyle().Background(accent).Foreground(lipgloss.Color("0")).Bold(true)
	dropStyle = lipgloss.NewStyle().Background(lipgloss.Color("238")).Foreground(accent).Bold(true)
	focusRow  = lipgloss.NewStyle().Background(lipgloss.Color("236"))
)

var widgetGroups = []struct {
	Name string
	IDs  []string
}{
	{"Session", []string{"model", "dir", "git", "userhost"}},
	{"Jauges", []string{"context", "limit5h", "limit7d"}},
	{"Activité", []string{"cost", "lines", "reset5h", "reset7d", "text"}},
}

// Zones fixes de l'inspecteur (réglages à bascule directe et ouverture des listes).
var inspIDs = []string{
	"insp:color", "insp:bar", "insp:mode", "insp:width", "insp:caps", "insp:text", "btn:remove",
	"opt:label:on", "opt:label:off", "opt:icon:yes", "opt:icon:no",
	"opt:version:yes", "opt:version:no", "opt:full:yes", "opt:full:no",
}

func (m model) selCell() *Cell {
	if m.sel == nil {
		return nil
	}
	return &m.cfg.Rows[m.sel.r][m.sel.c]
}

func (m model) layoutTab() string {
	grid := m.gridView()
	insp := m.inspector()
	switch {
	case m.width() >= 100: // widgets, grille, inspecteur côte à côte
		cols := []string{m.widgetsPanel(), "  ", grid}
		if insp != "" {
			cols = append(cols, "   ", insp)
		}
		return lipgloss.JoinHorizontal(lipgloss.Top, cols...)
	case m.width() >= 80: // un seul panneau : les widgets, ou les réglages de la sélection
		left := m.widgetsPanel()
		if insp != "" {
			left = insp
		}
		return lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", grid)
	}
	// Étroit : la grille seule, les réglages dessous ; les widgets via « a ».
	return lipgloss.JoinVertical(lipgloss.Left, grid, "", insp)
}

func (m model) widgetsPanel() string {
	head := heading.Render("WIDGETS")
	if m.drag != nil && m.drag.from != nil && (m.hover == "palette" || strings.HasPrefix(m.hover, "pal:")) {
		head = errStyle.Bold(true).Render("RELÂCHER : RETIRER")
	}
	lines := []string{head}
	for _, g := range widgetGroups {
		lines = append(lines, faint.Render(g.Name))
		for _, id := range g.IDs {
			lines = append(lines, "  "+zone.Mark("pal:"+id, label(id)))
		}
	}
	return zone.Mark("palette", lipgloss.NewStyle().Width(panelW).Render(strings.Join(lines, "\n")))
}

func (m model) gridView() string {
	edge := lipgloss.NewStyle().Foreground(edgeColor)
	n := len(m.cfg.Rows[0])
	colW := m.colW()
	seg := strings.Repeat("─", colW)
	rule := func(l, mid, r string) string {
		return "    " + edge.Render(l+strings.Repeat(seg+mid, n-1)+seg+r)
	}
	inner := n*(colW+1) - 1
	lines := []string{heading.Render("GRILLE"), rule("┌", "┬", "┐")}
	for r, row := range m.cfg.Rows {
		if r > 0 {
			lines = append(lines, rule("├", "┼", "┤"))
		}
		g := fmt.Sprintf("L%d", r+1)
		gst := faint
		if m.selRow != nil && *m.selRow == r {
			gst = selStyle
		}
		line := zone.Mark(fmt.Sprintf("row:%d", r), gst.Render(g)) + strings.Repeat(" ", 4-len(g)) + edge.Render("│")
		for c := range row {
			line += zone.Mark(cellID(r, c), m.cellText(r, c)) + edge.Render("│")
		}
		lines = append(lines, line)
		if m.gap(r) {
			lines = append(lines, "    "+edge.Render("│")+faint.Render(lipgloss.PlaceHorizontal(inner, lipgloss.Center, " ligne vide ", lipgloss.WithWhitespaceChars("┄")))+edge.Render("│"))
		}
	}
	lines = append(lines, rule("└", "┴", "┘"), "")
	lines = append(lines, "    "+lipgloss.JoinHorizontal(lipgloss.Top,
		zone.Mark("btn:+row", button.Render("+ ligne")), " ",
		zone.Mark("btn:-row", button.Render("− ligne")), "   ",
		zone.Mark("btn:+col", button.Render("+ colonne")), " ",
		zone.Mark("btn:-col", button.Render("− colonne"))))
	return strings.Join(lines, "\n")
}

func (m model) cellText(r, c int) string {
	cell := m.cfg.Rows[r][c]
	colW := m.colW()
	here := m.sel != nil && *m.sel == (pos{r, c})
	txt := lipgloss.PlaceHorizontal(colW, lipgloss.Center, "·")
	st := faint
	if cell.Widget != "" {
		t := label(cell.Widget)
		if cell.Widget == "text" && cell.Text != "" {
			t = "« " + cell.Text + " »"
		}
		if m.editing && here {
			t = "« " + cell.Text + "▏"
		}
		t = ansi.Truncate(" "+t, colW, "…")
		txt = t + strings.Repeat(" ", colW-ansi.StringWidth(t))
		st = colorOf(cell.Color)
	}
	switch {
	case m.drag != nil && m.hover == cellID(r, c):
		st = dropStyle
	case m.drag != nil && m.drag.from != nil && *m.drag.from == (pos{r, c}):
		st = faint
	case here:
		st = selStyle
	}
	return st.Render(txt)
}

// inspRow : une ligne de l'inspecteur, cliquable en entier (zone id).
func inspRow(id, name, value string, more bool) string {
	arrow := " "
	if more {
		arrow = lipgloss.NewStyle().Foreground(accent).Render("›")
	}
	v := ansi.Truncate(value, inspW-labelW-2, "…")
	row := faint.Width(labelW).Render(name) + v + strings.Repeat(" ", max(0, inspW-labelW-2-ansi.StringWidth(v))) + " " + arrow
	return zone.Mark(id, row)
}

func yesNo(b bool) string {
	if b {
		return "oui"
	}
	return "non"
}

func onOff(prefix string, on bool) string {
	if on {
		return prefix + "no"
	}
	return prefix + "yes"
}

// colorSummary : petite pastille et nom de la couleur (ou du schéma) d'une cellule.
func colorSummary(name string) string {
	if sc, ok := schemes[name]; ok {
		s := ""
		for _, c := range sc {
			s += lipgloss.NewStyle().Foreground(lipgloss.Color(c)).Render("█")
		}
		return s + " seuils " + name
	}
	if colorSpec(name) == "" {
		return faint.Render("auto")
	}
	return colorOf(name).Render("██") + " " + name
}

func (m model) inspector() string {
	if m.selRow != nil {
		r := *m.selRow
		return strings.Join([]string{
			heading.Render(fmt.Sprintf("LIGNE %d", r+1)), "",
			inspRow(fmt.Sprintf("gap:%d", r), "Ligne vide", "après cette ligne : "+yesNo(m.gap(r)), false),
		}, "\n")
	}
	c := m.selCell()
	if c == nil {
		return ""
	}
	where := faint.Render(fmt.Sprintf("  L%d · C%d", m.sel.r+1, m.sel.c+1))
	if c.Widget == "" {
		return heading.Render("CELLULE VIDE") + where + "\n\n" +
			faint.Render("Glisse un widget ici, ou ") + keyStyle.Render("a") + faint.Render(" pour choisir.")
	}
	rows := []string{heading.Render(strings.ToUpper(label(c.Widget))) + where, ""}
	rows = append(rows, inspRow("insp:color", "Couleur", colorSummary(c.Color), true))
	switch {
	case isGauge(c.Widget):
		used := 42.0
		fill, empty := bar(c.Bar, used, 4)
		w := c.Width
		if w == 0 {
			w = 10
		}
		mode := "utilisé"
		if c.Mode == "left" {
			mode = "restant"
		}
		caps := "aucun"
		for _, cp := range capStyles {
			if cp.ID == capsOf(*c) && cp.ID != "none" {
				caps = cp.Left + " " + cp.Right
			}
		}
		rows = append(rows,
			inspRow("insp:bar", "Barre", colorOf(c.Color).Render(fill)+faint.Render(empty)+" "+barLabel(c.Bar), true),
			inspRow("insp:mode", "Afficher", mode, true),
			inspRow("insp:width", "Largeur", fmt.Sprint(w), true),
			inspRow("insp:caps", "Embouts", caps, true),
			inspRow(onOff("opt:label:", !c.NoLabel), "Étiquette", yesNo(!c.NoLabel), false),
		)
	case c.Widget == "model":
		format := "Opus"
		if c.Icon {
			format = "✳ Opus"
		}
		rows = append(rows,
			inspRow(onOff("opt:icon:", c.Icon), "Format", format, false),
			inspRow(onOff("opt:version:", c.Version), "Version", yesNo(c.Version), false))
	case c.Widget == "dir":
		format := "nom du dossier"
		if c.Full {
			format = "chemin ~"
		}
		rows = append(rows, inspRow(onOff("opt:full:", c.Full), "Format", format, false))
	case c.Widget == "text":
		rows = append(rows, inspRow("insp:text", "Texte", "« "+c.Text+" »", true))
	}
	rows = append(rows, "", zone.Mark("btn:remove", keyStyle.Render("x")+faint.Render(" retirer de la grille")))
	return lipgloss.NewStyle().Width(inspW).Render(strings.Join(rows, "\n"))
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

// ---------- Listes de choix ----------

type popItem struct {
	ID, Label string
	On        bool
}

// popItems : les choix de la liste ouverte, dans l'ordre de navigation au clavier.
func (m model) popItems() []popItem {
	c := m.selCell()
	if c == nil {
		return nil
	}
	used := 42.0
	sample := func(mod func(*Cell)) string {
		s := *c
		s.NoLabel = true
		mod(&s)
		return gauge(s, &used, "")
	}
	var items []popItem
	switch m.pop {
	case "widget":
		for _, w := range widgets {
			items = append(items, popItem{"place:" + w.ID, w.Label, c.Widget == w.ID})
		}
	case "color":
		for _, s := range colorItems("auto") {
			items = append(items, popItem{"color:" + s.ID, s.ID, c.Color == s.ID})
		}
		if isGauge(c.Widget) {
			for _, name := range schemeOrder {
				items = append(items, popItem{"color:" + name, name, c.Color == name})
			}
		}
	case "bar":
		for _, b := range barStyles {
			id := b.ID
			items = append(items, popItem{"opt:bar:" + id, fmt.Sprintf("%-9s ", b.Label) + sample(func(s *Cell) { s.Bar = id }), barLabel(c.Bar) == b.Label})
		}
	case "mode":
		items = append(items,
			popItem{"opt:mode:", fmt.Sprintf("%-9s ", "utilisé") + sample(func(s *Cell) { s.Mode = "" }), c.Mode == ""},
			popItem{"opt:mode:left", fmt.Sprintf("%-9s ", "restant") + sample(func(s *Cell) { s.Mode = "left" }), c.Mode == "left"})
	case "width":
		cur := c.Width
		if cur == 0 {
			cur = 10
		}
		for _, w := range []int{5, 10, 15} {
			items = append(items, popItem{fmt.Sprintf("opt:width:%d", w), fmt.Sprintf("%-4d", w) + sample(func(s *Cell) { s.Width = w }), cur == w})
		}
	case "caps":
		for _, cp := range capStyles {
			id := cp.ID
			name := cp.Left + " " + cp.Right
			if id == "none" {
				name = "aucun"
			}
			items = append(items, popItem{"opt:caps:" + id, fmt.Sprintf("%-7s ", name) + sample(func(s *Cell) { s.Caps = id }), capsOf(*c) == id})
		}
	}
	return items
}

var popTitles = map[string]string{
	"widget": "Ajouter un widget", "color": "Couleur", "bar": "Style de barre",
	"mode": "Afficher", "width": "Largeur", "caps": "Embouts",
}

func (m model) popView() string {
	items := m.popItems()
	focus := ""
	if m.popIdx >= 0 && m.popIdx < len(items) {
		focus = items[m.popIdx].ID
	}
	var body string
	if m.pop == "color" {
		c := m.selCell()
		body = swatchGrid("color:", c.Color, strings.TrimPrefix(focus, "color:"), colorItems("auto"), 8)
		if isGauge(c.Widget) {
			var sw []swatch
			for _, name := range schemeOrder {
				sc := schemes[name]
				sw = append(sw, swatch{ID: name, Colors: sc[:]})
			}
			body += "\n\n" + heading.Render("SEUILS") + "\n\n" + swatchGrid("color:", c.Color, strings.TrimPrefix(focus, "color:"), sw, 4)
		}
	} else {
		w := 0
		for _, it := range items {
			w = max(w, ansi.StringWidth(it.Label))
		}
		var lines []string
		for _, it := range items {
			mark := "  "
			if it.On {
				mark = lipgloss.NewStyle().Foreground(accent).Render("✓ ")
			}
			line := mark + it.Label + strings.Repeat(" ", w-ansi.StringWidth(it.Label)) + " "
			if it.ID == focus {
				line = focusRow.Render(line)
			}
			lines = append(lines, zone.Mark(it.ID, line))
		}
		// Plus de 8 choix : deux colonnes, remplies de haut en bas.
		if rows := popRows(len(lines)); rows < len(lines) {
			body = lipgloss.JoinHorizontal(lipgloss.Top, strings.Join(lines[:rows], "\n"), "   ", strings.Join(lines[rows:], "\n"))
		} else {
			body = strings.Join(lines, "\n")
		}
	}
	st := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(accent).Padding(0, 1)
	return st.Render(title.Render(popTitles[m.pop]) + "\n\n" + body)
}

// popRows : nombre de lignes d'une liste de n choix (deux colonnes au-delà de 8).
func popRows(n int) int {
	if n > 8 {
		return (n + 1) / 2
	}
	return n
}

// cloneCfg copie la config en profondeur, pour essayer un choix sans le valider.
func cloneCfg(c Config) Config {
	rows := make([][]Cell, len(c.Rows))
	for i, r := range c.Rows {
		rows[i] = append([]Cell(nil), r...)
	}
	c.Rows = rows
	c.Gaps = append([]bool(nil), c.Gaps...)
	return c
}

// openPop ouvre une liste, le focus sur le choix actuel.
func (m *model) openPop(kind string) {
	m.pop, m.popIdx = kind, 0
	for i, it := range m.popItems() {
		if it.On {
			m.popIdx = i
		}
	}
}

// overlayBlock pose un bloc de plusieurs lignes sur bg, coin haut-gauche en (x, y).
func overlayBlock(bg, fg string, x, y int) string {
	for i, l := range strings.Split(fg, "\n") {
		bg = overlay(bg, l, x, y+i)
	}
	return bg
}
