package main

import (
	"strings"
	"testing"
)

func TestRender(t *testing.T) {
	cfg := Config{Rows: [][]Cell{
		{{Widget: "model", Color: "cyan"}, {}, {Widget: "dir"}},
		{{Widget: "cost"}, {Widget: "context", Bar: "text", NoLabel: true, Color: "gray"}, {Widget: "text", Text: "hi"}},
		{{}, {}},
	}}
	in := `{"model":{"display_name":"Opus"},"workspace":{"current_dir":"/home/u/proj"},
		"cost":{"total_cost_usd":0.01234},"context_window":{"used_percentage":8}}`
	// Grid: "Opus" takes the width of "$0.01", the empty column keeps the width of "8%".
	want := "\x1b[38;5;6mOpus\x1b[0m         proj\n$0.01 │ \x1b[38;5;8m8%\x1b[0m │ hi"
	if got := render([]byte(in), cfg); got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	// Blank line after rows 0 and 2: row 2 shows nothing, so no gap.
	cfg.Gaps, cfg.AirCols = []bool{true, false, true}, true
	want = "\x1b[38;5;6mOpus\x1b[0m                 proj\n\u200b\n$0.01   │   \x1b[38;5;8m8%\x1b[0m   │   hi"
	if got := render([]byte(in), cfg); got != want {
		t.Errorf("spaced: got %q, want %q", got, want)
	}
	cfg.Gaps, cfg.AirCols = nil, false

	// Model with the ✳ icon.
	icon := Config{Rows: [][]Cell{{{Widget: "model", Icon: true}}}}
	if got := render([]byte(in), icon); got != "✳ Opus" {
		t.Errorf("icon: got %q", got)
	}

	// Version taken from model.id, not duplicated if the name already has one.
	for _, tc := range []struct{ json, want string }{
		{`{"model":{"id":"claude-opus-5-5","display_name":"Opus"}}`, "✳ Opus 5.5"},
		{`{"model":{"id":"claude-3-5-sonnet-20241022","display_name":"Sonnet"}}`, "✳ Sonnet 3.5"},
		{`{"model":{"id":"claude-opus-4-1[1m]","display_name":"Opus 4.1"}}`, "✳ Opus 4.1"},
		{`{"model":{"display_name":"Opus"}}`, "✳ Opus"},
	} {
		icon.Rows[0][0].Version = true
		if got := render([]byte(tc.json), icon); got != tc.want {
			t.Errorf("version %s: got %q, want %q", tc.json, got, tc.want)
		}
	}

	// Chosen separator, and "space": spaces only.
	noGit0 := Config{Rows: [][]Cell{{{Widget: "model"}, {Widget: "dir"}}}, Sep: "›"}
	if got := render([]byte(in), noGit0); got != "Opus › proj" {
		t.Errorf("sep: got %q", got)
	}
	noGit0.Sep = "space"
	if got := render([]byte(in), noGit0); got != "Opus   proj" {
		t.Errorf("sep space: got %q", got)
	}

	// A widget without data (git outside a repo) is skipped without leaving a blank.
	noGit := Config{Rows: [][]Cell{{{Widget: "model"}, {Widget: "git"}, {Widget: "dir"}}}}
	if got := render([]byte(in), noGit); got != "Opus │ proj" {
		t.Errorf("widget absent: got %q", got)
	}

	// Empty or invalid JSON, or null percentage: only the free text remains.
	for _, in := range []string{"", "{", `{"context_window":{"used_percentage":null}}`} {
		if got := render([]byte(in), cfg); got != "hi" {
			t.Errorf("input %q: got %q, want %q", in, got, "hi")
		}
	}
}

func TestGridAlign(t *testing.T) {
	cfg := Config{Rows: [][]Cell{
		{{Widget: "text", Text: "Opus"}, {Widget: "text", Text: "dg-statuslines"}, {Widget: "text", Text: "main"}},
		{{Widget: "text", Text: "ctx 23%"}, {Widget: "text", Text: "5h"}, {Widget: "text", Text: "x"}},
		{{Widget: "text", Text: "a"}, {Widget: "spacer", Width: 8}, {}, {Widget: "text", Text: "b"}},
	}}
	want := "Opus    │ dg-statuslines │ main\n" +
		"ctx 23% │ 5h             │ x\n" +
		"a" + strings.Repeat(" ", 33) + "b" // 4th column: 7 + 3 + 14 (Spacer of 8, widened by "dg-statuslines") + 3 + 4 + 3
	if got := render(nil, cfg); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestAlign(t *testing.T) {
	cfg := Config{Rows: [][]Cell{
		{{Widget: "text", Text: "dg-statuslines"}, {Widget: "text", Text: "x"}},
		{{Widget: "text", Text: "main", Align: "right"}, {Widget: "text", Text: "y"}},
		{{Widget: "text", Text: "mid", Align: "center"}, {Widget: "text", Text: "z"}},
	}}
	want := "dg-statuslines │ x\n" +
		"          main │ y\n" +
		"     mid       │ z"
	if got := render(nil, cfg); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestFreeRow(t *testing.T) {
	cfg := Config{Free: []bool{true}, Rows: [][]Cell{
		{{Widget: "text", Text: "~/a/very/long/path"}, {Widget: "text", Text: "main"}},
		{{Widget: "text", Text: "Opus"}, {Widget: "text", Text: "$0.42"}},
		{{Widget: "text", Text: "ctx 23%"}, {Widget: "text", Text: "5h"}},
	}}
	// Row 1 is off the grid: the long path widens nothing, rows 2 and 3 align together.
	want := "~/a/very/long/path │ main\n" +
		"Opus    │ $0.42\n" +
		"ctx 23% │ 5h"
	if got := render(nil, cfg); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestCellSeparator(t *testing.T) {
	cfg := Config{Sep: "·", Rows: [][]Cell{
		{{Widget: "text", Text: "a", Sep: "›"}, {Widget: "text", Text: "b"}, {Widget: "text", Text: "c", Sep: "space"}, {Widget: "text", Text: "d"}},
		{{Widget: "text", Text: "w"}, {Widget: "text", Text: "x", Sep: "/"}, {Widget: "text", Text: "y"}, {Widget: "text", Text: "z"}},
	}}
	// Each cell picks what follows it; the others inherit "·". Same widths keep columns aligned.
	want := "a › b · c   d\nw · x / y · z"
	if got := render(nil, cfg); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestBorder(t *testing.T) {
	cfg := Config{Border: "rounded", Gaps: []bool{true}, Rows: [][]Cell{
		{{Widget: "text", Text: "Opus"}, {Widget: "text", Text: "main"}},
		{{Widget: "text", Text: "x"}},
	}}
	d := func(s string) string { return "\x1b[2m" + s + "\x1b[0m" }
	want := d("╭─────────────╮") + "\n" +
		d("│") + " Opus │ main " + d("│") + "\n" +
		d("│") + "             " + d("│") + "\n" +
		d("│") + " x           " + d("│") + "\n" +
		d("╰─────────────╯")
	if got := render(nil, cfg); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestTheme(t *testing.T) {
	// Hex -> truecolor, 256 index -> 38;5, palette name -> its index.
	if got := paint("x", colorSpec("#cba6f7")); got != "\x1b[38;2;203;166;247mx\x1b[0m" {
		t.Errorf("hex: got %q", got)
	}
	if got := paint("x", colorSpec("178")); got != "\x1b[38;5;178mx\x1b[0m" {
		t.Errorf("256: got %q", got)
	}
	// A gauge colored with a theme id uses that theme's thresholds.
	used := 90.0
	if got := gauge(Cell{Color: "dracula", Bar: "text", NoLabel: true}, &used, ""); got != "\x1b[38;2;255;85;85m90%\x1b[0m" {
		t.Errorf("theme thresholds: got %q", got)
	}
}

func TestIsLight(t *testing.T) {
	for spec, want := range map[string]bool{"#ffffff": true, "#1e1e2e": false, "4": false, "11": true, "220": true, "130": false, "250": true} {
		if isLight(spec) != want {
			t.Errorf("isLight(%s) = %v", spec, !want)
		}
	}
}

func TestBar(t *testing.T) {
	for _, tc := range []struct {
		style       string
		pct         float64
		fill, empty string
	}{
		{"blocks", 42, "████", "░░░░░░"},
		{"fine", 42, "████▎", "     "},
		{"line", 100, "━━━━━━━━━━", ""},
		{"dots", 0, "", "○○○○○○○○○○"},
		{"braille", 42, "⣿⣿⣿⣿⣄", "⣀⣀⣀⣀⣀"},
		{"ascii", 50, "#####", "-----"},
		{"text", 50, "", ""},
		{"squares", 42, "■■■■", "□□□□□□"},
		{"diamonds", 42, "◆◆◆◆", "◇◇◇◇◇◇"},
		{"segments", 42, "▰▰▰▰", "▱▱▱▱▱▱"},
		{"dashes", 42, "▬▬▬▬", "▭▭▭▭▭▭"},
		{"shade", 42, "▓▓▓▓", "░░░░░░"},
		{"slider", 42, "━━━━●", "─────"},
		{"slider", 0, "●", "─────────"},
		{"slider", 100, "━━━━━━━━━●", ""},
		{"ramp", 42, "▁▂▃▄", "······"},
		{"ramp", 100, "▁▂▃▄▄▅▆▇██", ""},
	} {
		f, e := bar(tc.style, tc.pct, 10)
		if f != tc.fill || e != tc.empty {
			t.Errorf("%s %v%%: got %q+%q, want %q+%q", tc.style, tc.pct, f, e, tc.fill, tc.empty)
		}
	}
}

func TestCaps(t *testing.T) {
	used := 50.0
	for _, tc := range []struct{ bar, caps, want string }{
		{"ascii", "", "\x1b[2m[\x1b[0m##\x1b[2m--\x1b[0m\x1b[2m]\x1b[0m 50%"},
		{"ascii", "none", "##\x1b[2m--\x1b[0m 50%"},
		{"blocks", "thin", "\x1b[2m▕\x1b[0m██\x1b[2m░░\x1b[0m\x1b[2m▏\x1b[0m 50%"},
		{"text", "parens", "50%"},
	} {
		c := Cell{Bar: tc.bar, Caps: tc.caps, Width: 4, NoLabel: true}
		if got := gauge(c, &used, ""); got != tc.want {
			t.Errorf("%s/%s: got %q, want %q", tc.bar, tc.caps, got, tc.want)
		}
	}
}

func TestGauge(t *testing.T) {
	used := 85.0
	c := Cell{Widget: "limit5h", Mode: "left", Width: 4, Color: "traffic"}
	// 15 % left, but red because 85 % is used.
	want := "\x1b[2m5h\x1b[0m \x1b[38;5;1m█\x1b[0m\x1b[2m░░░\x1b[0m \x1b[38;5;1m15%\x1b[0m"
	if got := gauge(c, &used, "5h"); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
