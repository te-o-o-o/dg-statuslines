package main

// Onglet Disposition : grille en cartes, widgets en pastilles, puis les réglages de la
// cellule (ou de la ligne) sélectionnée, rangés en sections. Tous les choix sont visibles.

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone"
)

var selStyle = lipgloss.NewStyle().Background(accent).Foreground(lipgloss.Color("0")).Bold(true)

var widgetGroups = []struct {
	Name string
	IDs  []string
}{
	{"Session", []string{"model", "dir", "git", "userhost"}},
	{"Jauges", []string{"context", "limit5h", "limit7d"}},
	{"Activité", []string{"cost", "lines", "reset5h", "reset7d", "text"}},
	{"Mise en page", []string{"spacer"}},
}

// optIDs : tous les choix cliquables des sections (zones fixes).
func optIDs() []string {
	ids := []string{"opt:mode:", "opt:mode:left", "opt:width:1", "opt:width:2", "opt:width:4",
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

// gapChipW : place du bouton « ligne vide » au bout de chaque ligne de la grille.
const gapChipW = 16

// cardW : largeur d'une carte, réduite pour que la grille et ses boutons tiennent dans le terminal.
func (m model) cardW() int {
	n := len(m.cfg.Rows[0])
	return min(17, max(10, (m.width()-2-4-(n-1)-gapChipW)/n))
}

func (m model) layoutTab() string {
	btns := lipgloss.JoinHorizontal(lipgloss.Top,
		toggle("btn:+row", "+ ligne", false), " ",
		toggle("btn:-row", "− ligne", false), "   ",
		toggle("btn:+col", "+ colonne", false), " ",
		toggle("btn:-col", "− colonne", false))
	parts := []string{m.titled(heading.Render("GRILLE")), "", m.cardsView(), "", "    " + btns, "", "", m.widgetsView()}
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
				cards = append(cards, " ")
			}
			cards = append(cards, m.card(r, c))
		}
		cards = append(cards, "  ", toggle(fmt.Sprintf("gap:%d", r), "↵ ligne vide", m.gap(r)))
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Center, cards...))
		if m.gap(r) {
			w := len(row)*(m.cardW()+1) - 1
			rows = append(rows, "    "+faint.Render(lipgloss.PlaceHorizontal(w, lipgloss.Center, " ligne vide ", lipgloss.WithWhitespaceChars("┄"))))
		}
	}
	return strings.Join(rows, "\n")
}

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
			text = faint.Render(ansi.Truncate(fmt.Sprintf("␣ vide %d", spacerWidth(cell)), w-2, "…"))
		}
	}
	st := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(edgeColor).Width(w - 2).Align(lipgloss.Center)
	if m.drag == nil && m.hover == cellID(r, c) {
		st = st.BorderForeground(accent) // survol
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
		head = errStyle.Bold(true).Render("RELÂCHER POUR RETIRER LE WIDGET")
	}
	head = m.titled(head) + "\n" + faint.Render("Glisse un widget vers la grille, ou ramène une cellule ici pour la retirer.")
	var groups []string
	for _, g := range widgetGroups {
		var chips []string
		for i, id := range g.IDs {
			if i > 0 {
				chips = append(chips, " ")
			}
			edge := edgeColor
			if m.drag == nil && m.hover == "pal:"+id || m.drag != nil && m.drag.from == nil && m.drag.cell.Widget == id {
				edge = accent // survolé, ou en cours de glisser
			}
			chips = append(chips, zone.Mark("pal:"+id, lipgloss.NewStyle().Border(dashedBorder).BorderForeground(edge).Padding(0, 1).Render(label(id))))
		}
		// Intitulé du groupe sur la ligne du milieu des boutons.
		groups = append(groups, lipgloss.JoinHorizontal(lipgloss.Center, faint.Width(labelW).Render(g.Name), lipgloss.JoinHorizontal(lipgloss.Top, chips...)))
	}
	// Groupes collés : les contours des boutons suffisent à séparer les rangées.
	return zone.Mark("palette", head+"\n\n"+strings.Join(groups, "\n"))
}

// dashedBorder : contour en tirets des boutons de la liste des widgets (« à faire glisser »).
var dashedBorder = lipgloss.Border{
	Top: "╌", Bottom: "╌", Left: "╎", Right: "╎",
	TopLeft: "╭", TopRight: "╮", BottomLeft: "╰", BottomRight: "╯",
}

// sections : les sections de réglages selon le widget de la cellule.
func sections(c *Cell) []string {
	switch {
	case isGauge(c.Widget):
		return []string{"Couleur", "Barre", "Affichage"}
	case c.Widget == "model", c.Widget == "dir":
		return []string{"Couleur", "Format"}
	case c.Widget == "text":
		return []string{"Couleur", "Texte"}
	case c.Widget == "spacer":
		return []string{"Largeur"}
	}
	return []string{"Couleur"}
}

// curSection : la section ouverte, la première si celle choisie n'existe pas pour ce widget.
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
	if m.selRow != nil {
		r := *m.selRow
		return m.titled(heading.Render(fmt.Sprintf("LIGNE %d", r+1))) + "\n\n" +
			field("Ligne vide", toggle(fmt.Sprintf("gap:%d", r), "après cette ligne", m.gap(r)))
	}
	c := m.selCell()
	if c == nil {
		return ""
	}
	where := faint.Render(fmt.Sprintf("  ligne %d · colonne %d", m.sel.r+1, m.sel.c+1))
	if c.Widget == "" {
		return m.titled(heading.Render("CELLULE VIDE")+where) + "\n\n" + faint.Render("Glisse un widget depuis la liste au-dessus.")
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
	case "Couleur":
		// Le nom de la section suffit : les carrés s'alignent à gauche, sans intitulé.
		body = append(body, swatchGrid("color:", c.Color, "", colorItems("auto"), 8))
		if isGauge(c.Widget) {
			var sw []swatch
			for _, name := range schemeOrder {
				sc := schemes[name]
				sw = append(sw, swatch{ID: name, Colors: sc[:]})
			}
			body = append(body, faint.Render("Seuils")+"\n\n"+swatchGrid("color:", c.Color, "", sw, 4))
		}
	case "Barre":
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
		body = append(body, field("Barre", strings.Join(rows, "\n\n")))
	case "Affichage":
		w := c.Width
		if w == 0 {
			w = 10
		}
		var caps []opt
		for _, cp := range capStyles {
			name := cp.Left + " " + cp.Right
			if cp.ID == "none" {
				name = "aucun"
			}
			caps = append(caps, opt{"opt:caps:" + cp.ID, name, capsOf(*c) == cp.ID})
		}
		body = append(body,
			field("Afficher", chips(opt{"opt:mode:", "utilisé", c.Mode == ""}, opt{"opt:mode:left", "restant", c.Mode == "left"})),
			field("Largeur", chips(opt{"opt:width:5", "5", w == 5}, opt{"opt:width:10", "10", w == 10}, opt{"opt:width:15", "15", w == 15})),
			field("Embouts", chips(caps...)),
			field("Étiquette", chips(opt{"opt:label:on", "oui", !c.NoLabel}, opt{"opt:label:off", "non", c.NoLabel})))
	case "Format":
		if c.Widget == "model" {
			body = append(body,
				field("Format", chips(opt{"opt:icon:no", "Opus", !c.Icon}, opt{"opt:icon:yes", "✳ Opus", c.Icon})),
				field("Version", chips(opt{"opt:version:no", "non", !c.Version}, opt{"opt:version:yes", "oui", c.Version})))
		} else {
			body = append(body, field("Format", chips(opt{"opt:full:no", "nom du dossier", !c.Full}, opt{"opt:full:yes", "chemin ~", c.Full})))
		}
	case "Largeur":
		w := spacerWidth(*c)
		var ws []opt
		for _, n := range []int{1, 2, 4, 8, 16} {
			ws = append(ws, opt{fmt.Sprintf("opt:width:%d", n), fmt.Sprint(n), w == n})
		}
		body = append(body, field("Largeur", chips(ws...)+faint.Render("  espaces réservés dans la colonne")))
	case "Texte":
		body = append(body, field("Texte", "« "+c.Text+" »   "+toggle("insp:text", "modifier", m.editing)))
	}

	return m.titled(heading.Render(strings.ToUpper(label(c.Widget)))+where) + "\n\n" +
		strings.Join(tabs, "    ") + "\n\n" + strings.Join(body, "\n\n")
}

// titled prolonge un titre de bloc par un filet gris jusqu'au bord droit.
func (m model) titled(title string) string {
	rule := m.width() - 6 - ansi.StringWidth(title) // retrait de 2, marge droite de 2, espaces
	if rule < 2 {
		return title
	}
	return title + " " + lipgloss.NewStyle().Foreground(lipgloss.Color("237")).Render(strings.Repeat("─", rule))
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
