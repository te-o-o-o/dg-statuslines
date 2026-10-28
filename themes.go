package main

// Catalogue of looks: colors, threshold schemes, themes, borders, separators,
// bar caps and bar styles.

// Available colors: name -> index in the 256-color palette.
// 15 ANSI colors (normal then bright) and 9 shades from the 256 palette.
var colors = []struct {
	Name  string
	Index int
}{
	{"red", 1}, {"green", 2}, {"yellow", 3}, {"blue", 4},
	{"magenta", 5}, {"cyan", 6}, {"white", 7}, {"gray", 8},
	{"bright-red", 9}, {"bright-green", 10}, {"bright-yellow", 11}, {"bright-blue", 12},
	{"bright-magenta", 13}, {"bright-cyan", 14}, {"bright-white", 15},
	{"orange", 208}, {"gold", 220}, {"lime", 154}, {"teal", 37}, {"sky", 117},
	{"purple", 141}, {"pink", 212}, {"salmon", 210}, {"brown", 130},
}

// Gauge threshold schemes: color below 50 %, below 80 %, above (on usage).
// Each theme also adds its own scheme, under its id.
var schemes = map[string][3]string{
	"traffic": {"2", "3", "1"},
	"cyans":   {"123", "51", "30"},
	"fire":    {"220", "208", "196"},
	"violets": {"183", "141", "93"},
	// Funky ones.
	"neon":      {"#39ff14", "#ffff33", "#ff073a"},
	"vaporwave": {"#00ffff", "#ff71ce", "#b967ff"},
	"aurora":    {"#5fffaf", "#5fafff", "#d75fff"},
	"lava":      {"#ffd700", "#ff5f00", "#d70000"},
	"candy":     {"#87d7ff", "#ff87d7", "#ff5f87"},
	"matrix":    {"#00ff41", "#00cc33", "#008f11"},
}

var schemeOrder = []string{"traffic", "cyans", "fire", "violets", "neon", "vaporwave", "aurora", "lava", "candy", "matrix"}

// Themes: one color per widget, gauge thresholds and border color.
// Colors as 256 index or #rrggbb (official theme palettes).
type theme struct {
	ID, Label                                        string
	Model, Dir, Git, User, Cost, Reset, Text, Border string
	Gauge                                            [3]string
}

var themes = []theme{
	{"classic", "Classic", "51", "12", "178", "2", "178", "123", "", "30", [3]string{"123", "51", "30"}},
	{"catppuccin", "Catppuccin", "#cba6f7", "#89b4fa", "#a6e3a1", "#fab387", "#f9e2af", "#89dceb", "#b4befe", "#6c7086", [3]string{"#a6e3a1", "#f9e2af", "#f38ba8"}},
	{"nord", "Nord", "#88c0d0", "#81a1c1", "#a3be8c", "#b48ead", "#ebcb8b", "#8fbcbb", "#d8dee9", "#4c566a", [3]string{"#a3be8c", "#ebcb8b", "#bf616a"}},
	{"dracula", "Dracula", "#bd93f9", "#8be9fd", "#50fa7b", "#ff79c6", "#f1fa8c", "#ffb86c", "#f8f8f2", "#6272a4", [3]string{"#50fa7b", "#ffb86c", "#ff5555"}},
	{"tokyonight", "Tokyo Night", "#bb9af7", "#7aa2f7", "#9ece6a", "#7dcfff", "#e0af68", "#73daca", "#c0caf5", "#565f89", [3]string{"#9ece6a", "#e0af68", "#f7768e"}},
	{"gruvbox", "Gruvbox", "#d3869b", "#83a598", "#b8bb26", "#8ec07c", "#fabd2f", "#fe8019", "#ebdbb2", "#928374", [3]string{"#b8bb26", "#fabd2f", "#fb4934"}},
	{"rosepine", "Rosé Pine", "#c4a7e7", "#9ccfd8", "#31748f", "#ebbcba", "#f6c177", "#ebbcba", "#e0def4", "#6e6a86", [3]string{"#9ccfd8", "#f6c177", "#eb6f92"}},
	{"solarized", "Solarized", "#6c71c4", "#268bd2", "#859900", "#2aa198", "#b58900", "#cb4b16", "#93a1a1", "#586e75", [3]string{"#859900", "#b58900", "#dc322f"}},
	{"onedark", "One Dark", "#c678dd", "#61afef", "#98c379", "#56b6c2", "#e5c07b", "#d19a66", "#abb2bf", "#5c6370", [3]string{"#98c379", "#e5c07b", "#e06c75"}},
	{"monokai", "Monokai", "#ae81ff", "#66d9ef", "#a6e22e", "#fd971f", "#e6db74", "#fd971f", "#f8f8f2", "#75715e", [3]string{"#a6e22e", "#e6db74", "#f92672"}},
	{"everforest", "Everforest", "#d699b6", "#7fbbb3", "#a7c080", "#83c092", "#dbbc7f", "#e69875", "#d3c6aa", "#859289", [3]string{"#a7c080", "#dbbc7f", "#e67e80"}},
	{"kanagawa", "Kanagawa", "#957fb8", "#7e9cd8", "#98bb6c", "#7aa89f", "#e6c384", "#ffa066", "#dcd7ba", "#727169", [3]string{"#98bb6c", "#e6c384", "#e46876"}},
	{"ayu", "Ayu", "#d2a6ff", "#59c2ff", "#aad94c", "#95e6cb", "#e6b450", "#ff8f40", "#bfbdb6", "#636a72", [3]string{"#aad94c", "#e6b450", "#f07178"}},
	{"githubdark", "GitHub Dark", "#bc8cff", "#58a6ff", "#3fb950", "#39c5cf", "#d29922", "#db6d28", "#c9d1d9", "#8b949e", [3]string{"#3fb950", "#d29922", "#f85149"}},
	{"mono", "Mono", "#e4e4e4", "#bcbcbc", "#9e9e9e", "#c6c6c6", "#dadada", "#a8a8a8", "#d0d0d0", "#585858", [3]string{"#9e9e9e", "#d0d0d0", "#ff5f5f"}},
}

func init() {
	for _, t := range themes {
		schemes[t.ID] = t.Gauge
	}
}

// color returns a widget's color in the theme ("" = widget default color).
func (t theme) color(widget string) string {
	switch widget {
	case "model", "version", "effort", "fast", "thinking":
		return t.Model
	case "dir", "project", "repo", "added":
		return t.Dir
	case "git", "worktree", "pr":
		return t.Git
	case "userhost", "session", "agent":
		return t.User
	case "cost", "tokens", "ctxsize", "cache":
		return t.Cost
	case "reset5h", "reset7d", "duration", "apitime":
		return t.Reset
	case "text", "style", "vim", "over200k":
		return t.Text
	}
	if isGauge(widget) {
		return t.ID // theme threshold scheme
	}
	return ""
}

// Available separators; "space" = spaces only.
// Status line borders: top-left, top-right, bottom-left, bottom-right corners, horizontal, vertical.
// B and R are the bottom and right edges when they differ from H and V (block frame).
var borders = []struct{ ID, TL, TR, BL, BR, H, V, B, R string }{
	{"rounded", "╭", "╮", "╰", "╯", "─", "│", "", ""},
	{"square", "┌", "┐", "└", "┘", "─", "│", "", ""},
	{"double", "╔", "╗", "╚", "╝", "═", "║", "", ""},
	{"heavy", "┏", "┓", "┗", "┛", "━", "┃", "", ""},
	{"dashed", "╭", "╮", "╰", "╯", "╌", "╎", "", ""},
	{"dotted", "┌", "┐", "└", "┘", "┈", "┊", "", ""},
	{"dots", "·", "·", "·", "·", "·", "·", "", ""},
	{"block", "▛", "▜", "▙", "▟", "▀", "▌", "▄", "▐"},
	{"corners", "┌", "┐", "└", "┘", " ", " ", "", ""},
	{"ascii", "+", "+", "+", "+", "-", "|", "", ""},
}

var separators = []string{"│", "|", "┃", "·", "•", "/", "›", "»", "◆", "⋮", "space"}

// Bar caps: id -> left, right ("none" = no caps).
var capStyles = []struct{ ID, Left, Right string }{
	{"none", "", ""}, {"brackets", "[", "]"}, {"thin", "▕", "▏"}, {"parens", "(", ")"}, {"angles", "⟨", "⟩"},
}

// capsOf resolves the default: ASCII keeps its historical brackets.
func capsOf(c Cell) string {
	if c.Caps == "" {
		if c.Bar == "ascii" {
			return "brackets"
		}
		return "none"
	}
	return c.Caps
}

var barStyles = []struct{ ID, Label string }{
	{"blocks", "Blocks"}, {"fine", "Fine"}, {"line", "Line"}, {"dots", "Dots"},
	{"braille", "Braille"}, {"ascii", "ASCII"}, {"text", "Text"},
	{"squares", "Squares"}, {"diamonds", "Diamonds"}, {"segments", "Segments"}, {"dashes", "Dashes"},
	{"shade", "Shade"}, {"slider", "Slider"}, {"ramp", "Ramp"},
}

// recolor applies a theme: every widget gets its colour, and the border its own.
func recolor(cfg *Config, t theme) {
	cfg.Theme, cfg.BorderColor = t.ID, t.Border
	for r := range cfg.Rows {
		for c := range cfg.Rows[r] {
			if w := cfg.Rows[r][c].Widget; w != "" {
				cfg.Rows[r][c].Color = t.color(w)
			}
		}
	}
}
