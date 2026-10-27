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
	// Grille : « Opus » prend la largeur de « $0.01 », la colonne vide garde la largeur de « 8% ».
	want := "\x1b[38;5;6mOpus\x1b[0m         proj\n$0.01 │ \x1b[38;5;8m8%\x1b[0m │ hi"
	if got := render([]byte(in), cfg); got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	// Ligne vide après les lignes 0 et 2 : la ligne 2 n'affiche rien, donc pas d'espace.
	cfg.Gaps, cfg.AirCols = []bool{true, false, true}, true
	want = "\x1b[38;5;6mOpus\x1b[0m                 proj\n\u200b\n$0.01   │   \x1b[38;5;8m8%\x1b[0m   │   hi"
	if got := render([]byte(in), cfg); got != want {
		t.Errorf("aéré: got %q, want %q", got, want)
	}
	cfg.Gaps, cfg.AirCols = nil, false

	// Modèle avec l'icône ✳.
	icon := Config{Rows: [][]Cell{{{Widget: "model", Icon: true}}}}
	if got := render([]byte(in), icon); got != "✳ Opus" {
		t.Errorf("icon: got %q", got)
	}

	// Version tirée de model.id, sans doublon si le nom en contient déjà une.
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

	// Séparateur choisi, et "space" : seulement des espaces.
	noGit0 := Config{Rows: [][]Cell{{{Widget: "model"}, {Widget: "dir"}}}, Sep: "›"}
	if got := render([]byte(in), noGit0); got != "Opus › proj" {
		t.Errorf("sep: got %q", got)
	}
	noGit0.Sep = "space"
	if got := render([]byte(in), noGit0); got != "Opus  proj" {
		t.Errorf("sep space: got %q", got)
	}

	// Un widget sans donnée (git hors dépôt) est sauté sans laisser de blanc.
	noGit := Config{Rows: [][]Cell{{{Widget: "model"}, {Widget: "git"}, {Widget: "dir"}}}}
	if got := render([]byte(in), noGit); got != "Opus │ proj" {
		t.Errorf("widget absent: got %q", got)
	}

	// JSON vide, invalide, ou pourcentage null : seul le texte libre reste.
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
		"a" + strings.Repeat(" ", 33) + "b" // 4e colonne : 7 + 3 + 14 (Vide de 8, élargi par « dg-statuslines ») + 3 + 4 + 3
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
	// Hex -> truecolor, index 256 -> 38;5, nom de palette -> son index.
	if got := paint("x", colorSpec("#cba6f7")); got != "\x1b[38;2;203;166;247mx\x1b[0m" {
		t.Errorf("hex: got %q", got)
	}
	if got := paint("x", colorSpec("178")); got != "\x1b[38;5;178mx\x1b[0m" {
		t.Errorf("256: got %q", got)
	}
	// Une jauge colorée avec l'id d'un thème utilise les seuils de ce thème.
	used := 90.0
	if got := gauge(Cell{Color: "dracula", Bar: "text", NoLabel: true}, &used, ""); got != "\x1b[38;2;255;85;85m90%\x1b[0m" {
		t.Errorf("seuils du thème: got %q", got)
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
	// 15 % restant, mais rouge car 85 % utilisés.
	want := "\x1b[2m5h\x1b[0m \x1b[38;5;1m█\x1b[0m\x1b[2m░░░\x1b[0m \x1b[38;5;1m15%\x1b[0m"
	if got := gauge(c, &used, "5h"); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
