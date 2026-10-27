package main

// Random layouts: chance within rules, so every roll stays readable.

import "math/rand/v2"

// randomConfig draws a theme, 1 to 3 rows of 3 to 5 distinct widgets, gauges grouped
// on one row with a shared bar style, a sober separator and sometimes a border.
func randomConfig(r *rand.Rand) Config {
	pick := func(xs []string) string { return xs[r.IntN(len(xs))] }
	chance := func(p float64) bool { return r.Float64() < p }

	others := []string{"model", "dir", "git", "userhost", "cost", "lines"}
	r.Shuffle(len(others), func(i, j int) { others[i], others[j] = others[j], others[i] })
	// Resets keep their order (5h then 7d) and follow the gauges, or close the line.
	var resets []Cell
	for _, w := range []string{"reset5h", "reset7d"} {
		if chance(0.4) {
			resets = append(resets, Cell{Widget: w})
		}
	}
	// The first row usually opens on the model or the directory.
	if chance(0.7) {
		for i, w := range others {
			if w == "model" || w == "dir" {
				others[0], others[i] = others[i], others[0]
				break
			}
		}
	}
	take := func() (Cell, bool) {
		if len(others) == 0 {
			return Cell{}, false
		}
		w := others[0]
		others = others[1:]
		return Cell{Widget: w}, true
	}

	// Gauges share one bar style, width, mode and caps.
	var gauges []Cell
	if n := r.IntN(4); n > 0 {
		style := Cell{Bar: pick([]string{"blocks", "fine", "line", "dots", "braille", "squares", "diamonds", "segments", "dashes", "shade", "slider", "ramp"}),
			Width: []int{5, 8, 10}[r.IntN(3)]}
		if chance(0.3) {
			style.Caps = pick([]string{"brackets", "thin", "parens", "angles"})
		}
		left := chance(0.5)
		for _, w := range []string{"context", "limit5h", "limit7d"}[:n] {
			g := style
			g.Widget = w
			if left && w != "context" {
				g.Mode = "left"
			}
			gauges = append(gauges, g)
		}
	}

	nrows := 1 + r.IntN(3)
	gaugeRow := r.IntN(nrows)
	var rows [][]Cell
	for i := range nrows {
		var row []Cell
		if i == gaugeRow {
			row = append(row, gauges...)
			if len(gauges) > 0 {
				row, resets = append(row, resets...), nil
			}
		}
		for size := 3 + r.IntN(3); len(row) < size; {
			c, ok := take()
			if !ok {
				break
			}
			row = append(row, c)
		}
		if len(row) > 0 {
			rows = append(rows, row)
		}
	}
	if len(resets) > 0 && len(rows) > 0 {
		rows[len(rows)-1] = append(rows[len(rows)-1], resets...)
	}

	cfg := Config{Sep: pick([]string{"", "·", "•", "›", "/", "space"})}
	if chance(1.0 / 3) {
		cfg.Border = pick([]string{"rounded", "square"})
	}
	if len(rows) > 1 && chance(0.5) {
		cfg.Gaps = make([]bool, len(rows)-1)
		for i := range cfg.Gaps {
			cfg.Gaps[i] = true
		}
	}
	lastRight := chance(0.3)
	for _, row := range rows {
		for k := range row {
			switch row[k].Widget {
			case "model":
				row[k].Icon, row[k].Version = chance(0.5), chance(0.5)
			case "dir":
				row[k].Full = chance(0.3)
			}
		}
		if lastRight {
			row[len(row)-1].Align = "right"
		}
	}
	return grid(themes[r.IntN(len(themes))].ID, cfg, rows...)
}

// cloneConfig copies a config deeply, so editing the current one never changes the history.
func cloneConfig(c Config) Config {
	rows := make([][]Cell, len(c.Rows))
	for i, row := range c.Rows {
		rows[i] = append([]Cell(nil), row...)
	}
	c.Rows = rows
	c.Gaps = append([]bool(nil), c.Gaps...)
	return c
}

// roll draws a new layout and appends it to the history (dropping any rolls after the current one).
func (m *model) roll() {
	if m.rng == nil {
		m.rng = rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	}
	c := randomConfig(m.rng)
	m.rolls = append(m.rolls[:m.rollIdx+min(1, len(m.rolls))], c)
	m.rollIdx = len(m.rolls) - 1
	m.cfg, m.sel, m.selSep = cloneConfig(c), nil, nil
}

// prevRoll goes back to the previous roll, if any.
func (m *model) prevRoll() bool {
	if m.rollIdx == 0 || len(m.rolls) == 0 {
		return false
	}
	m.rollIdx--
	m.cfg, m.sel, m.selSep = cloneConfig(m.rolls[m.rollIdx]), nil, nil
	return true
}
