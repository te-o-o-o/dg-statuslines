package main

// Style tab: app appearance, themes, separator, border and spacing.

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

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
	recolor(&m.cfg, t)
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
func swatchGrid(prefix, cur string, items []swatch, perRow int) string {
	var squares, chips []string
	for _, it := range items {
		var cell string
		switch {
		case len(it.Colors) == 0:
			cell = toggleStyle(it.ID == cur).Render(it.Label)
			chips = append(chips, zone.Mark(prefix+it.ID, cell))
			continue
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

// swatchCell: 2 background characters per color; mark (✓ chosen) on the middle color.
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
		parts = append(parts, "", field("", swatchGrid("bcolor:", m.cfg.BorderColor, colorItems("gray"), 8)))
	}
	parts = append(parts, "", field("Spacing",
		toggle("btn:airrows", "blank line between rows", m.allGaps())+" "+
			toggle("btn:aircols", "spaced columns", m.cfg.AirCols)))
	return strings.Join(parts, "\n")
}
