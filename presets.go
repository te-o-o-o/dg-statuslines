package main

// Ready-made configurations: a grid, its widget options and a theme.

type preset struct {
	ID, Label string
	build     func() Config
}

var presets = []preset{
	{"minimal", "Minimal", func() Config {
		return grid("catppuccin", Config{Sep: "·"},
			[]Cell{{Widget: "model", Icon: true, Version: true}, {Widget: "dir"}, {Widget: "git"}})
	}},
	{"classic", "Classic", func() Config {
		return grid("classic", Config{Gaps: []bool{true, true}},
			// The full path goes last: in an aligned grid, a wide column pushes the ones after it.
			[]Cell{{Widget: "userhost"}, {Widget: "git"}, {Widget: "dir", Full: true}},
			[]Cell{{Widget: "context"}, {Widget: "limit5h", Mode: "left"}, {Widget: "limit7d", Mode: "left"}},
			[]Cell{{Widget: "model", Icon: true}, {Widget: "reset5h"}, {Widget: "reset7d"}})
	}},
	{"compact", "Compact", func() Config {
		bar := func(w string, mode string) Cell { return Cell{Widget: w, Bar: "segments", Width: 5, Mode: mode} }
		return grid("tokyonight", Config{},
			[]Cell{{Widget: "model", Version: true}, bar("context", ""), bar("limit5h", "left"), bar("limit7d", "left"), {Widget: "cost"}})
	}},
	{"dashboard", "Dashboard", func() Config {
		return grid("dracula", Config{Border: "rounded"},
			[]Cell{{Widget: "model", Icon: true, Version: true}, {Widget: "dir"}, {Widget: "git"}},
			[]Cell{{Widget: "context", Bar: "slider"}, {Widget: "limit5h", Bar: "slider", Mode: "left"}, {Widget: "cost"}})
	}},
	{"developer", "Developer", func() Config {
		return grid("gruvbox", Config{},
			[]Cell{{Widget: "dir"}, {Widget: "git"}, {Widget: "lines"}, {Widget: "cost"}})
	}},
	{"quota", "Quota watch", func() Config {
		return grid("everforest", Config{},
			[]Cell{{Widget: "limit5h", Bar: "dots", Mode: "left"}, {Widget: "reset5h"}},
			[]Cell{{Widget: "limit7d", Bar: "dots", Mode: "left"}, {Widget: "reset7d"}})
	}},
}

// grid builds a preset: rows padded to the same width, theme colors applied.
func grid(themeID string, cfg Config, rows ...[]Cell) Config {
	n := 0
	for _, r := range rows {
		n = max(n, len(r))
	}
	for _, r := range rows {
		cfg.Rows = append(cfg.Rows, append(r, make([]Cell, n-len(r))...))
	}
	if t, ok := themeByID(themeID); ok {
		recolor(&cfg, t)
	}
	return cfg
}
