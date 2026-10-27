package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

type Cell struct {
	Widget  string `json:"widget,omitempty"`
	Color   string `json:"color,omitempty"` // couleur fixe, ou schéma à seuils pour les jauges
	Text    string `json:"text,omitempty"`
	Bar     string `json:"bar,omitempty"`   // style de barre des jauges ("" = blocks)
	Mode    string `json:"mode,omitempty"`  // jauges : "" = utilisé, "left" = restant
	Width   int    `json:"width,omitempty"` // jauges : 0 = 10
	NoLabel bool   `json:"no_label,omitempty"`
	Full    bool   `json:"full,omitempty"`    // répertoire : chemin complet avec ~
	Caps    string `json:"caps,omitempty"`    // jauges : embouts ("" = [ ] pour ASCII, aucun sinon)
	Icon    bool   `json:"icon,omitempty"`    // modèle : préfixe ✳ comme l'ancien statusline-command.sh
	Version bool   `json:"version,omitempty"` // modèle : ajoute la version tirée de model.id
}

type Config struct {
	Rows        [][]Cell `json:"rows"`
	Gaps        []bool   `json:"gaps,omitempty"`         // Gaps[i] : ligne vide après la ligne i
	AirCols     bool     `json:"air_cols,omitempty"`     // plus d'espace entre les cellules
	Sep         string   `json:"sep,omitempty"`          // séparateur entre cellules ("" = │, "space" = aucun)
	Border      string   `json:"border,omitempty"`       // contour de toute la status line ("" = aucun)
	BorderColor string   `json:"border_color,omitempty"` // "" = gris atténué
	Theme       string   `json:"theme,omitempty"`        // dernier thème appliqué, sert aussi aux widgets ajoutés ensuite
}

// Widget id -> libellé affiché dans le builder (ordre de la palette).
var widgets = []struct{ ID, Label string }{
	{"model", "Modèle"},
	{"dir", "Répertoire"},
	{"git", "Branche git"},
	{"userhost", "user@host"},
	{"cost", "Coût"},
	{"lines", "Lignes +/−"},
	{"context", "Contexte"},
	{"limit5h", "Limite 5h"},
	{"limit7d", "Limite 7j"},
	{"reset5h", "Reset 5h"},
	{"reset7d", "Reset 7j"},
	{"text", "Texte"},
}

func isGauge(w string) bool { return w == "context" || w == "limit5h" || w == "limit7d" }

// Couleurs proposées : nom -> index de la palette 256 couleurs.
// 15 couleurs ANSI (normales puis vives) et 9 nuances de la palette 256.
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

// Schémas à seuils des jauges : couleur sous 50 %, sous 80 %, au-delà (sur l'utilisé).
// Chaque thème y ajoute aussi son propre schéma, sous son id.
var schemes = map[string][3]string{
	"traffic": {"2", "3", "1"},
	"cyans":   {"123", "51", "30"},
	"fire":    {"220", "208", "196"},
	"violets": {"183", "141", "93"},
}

var schemeOrder = []string{"traffic", "cyans", "fire", "violets"}

// Thèmes : une couleur par widget, les seuils des jauges et la couleur du contour.
// Couleurs en index 256 ou en #rrggbb (palettes officielles des thèmes).
type theme struct {
	ID, Label                                        string
	Model, Dir, Git, User, Cost, Reset, Text, Border string
	Gauge                                            [3]string
}

var themes = []theme{
	{"classic", "Classique", "51", "12", "178", "2", "178", "123", "", "30", [3]string{"123", "51", "30"}},
	{"catppuccin", "Catppuccin", "#cba6f7", "#89b4fa", "#a6e3a1", "#fab387", "#f9e2af", "#89dceb", "#b4befe", "#6c7086", [3]string{"#a6e3a1", "#f9e2af", "#f38ba8"}},
	{"nord", "Nord", "#88c0d0", "#81a1c1", "#a3be8c", "#b48ead", "#ebcb8b", "#8fbcbb", "#d8dee9", "#4c566a", [3]string{"#a3be8c", "#ebcb8b", "#bf616a"}},
	{"dracula", "Dracula", "#bd93f9", "#8be9fd", "#50fa7b", "#ff79c6", "#f1fa8c", "#ffb86c", "#f8f8f2", "#6272a4", [3]string{"#50fa7b", "#ffb86c", "#ff5555"}},
	{"tokyonight", "Tokyo Night", "#bb9af7", "#7aa2f7", "#9ece6a", "#7dcfff", "#e0af68", "#73daca", "#c0caf5", "#565f89", [3]string{"#9ece6a", "#e0af68", "#f7768e"}},
	{"gruvbox", "Gruvbox", "#d3869b", "#83a598", "#b8bb26", "#8ec07c", "#fabd2f", "#fe8019", "#ebdbb2", "#928374", [3]string{"#b8bb26", "#fabd2f", "#fb4934"}},
	{"rosepine", "Rosé Pine", "#c4a7e7", "#9ccfd8", "#31748f", "#ebbcba", "#f6c177", "#ebbcba", "#e0def4", "#6e6a86", [3]string{"#9ccfd8", "#f6c177", "#eb6f92"}},
}

func init() {
	for _, t := range themes {
		schemes[t.ID] = t.Gauge
	}
}

// color renvoie la couleur d'un widget dans le thème ("" = couleur par défaut du widget).
func (t theme) color(widget string) string {
	switch widget {
	case "model":
		return t.Model
	case "dir":
		return t.Dir
	case "git":
		return t.Git
	case "userhost":
		return t.User
	case "cost":
		return t.Cost
	case "reset5h", "reset7d":
		return t.Reset
	case "text":
		return t.Text
	}
	if isGauge(widget) {
		return t.ID // schéma à seuils du thème
	}
	return ""
}

// Séparateurs proposés ; "space" = seulement des espaces.
// Contours de la status line : coins haut-gauche, haut-droit, bas-gauche, bas-droit, horizontal, vertical.
var borders = []struct{ ID, TL, TR, BL, BR, H, V string }{
	{"rounded", "╭", "╮", "╰", "╯", "─", "│"},
	{"square", "┌", "┐", "└", "┘", "─", "│"},
	{"double", "╔", "╗", "╚", "╝", "═", "║"},
	{"heavy", "┏", "┓", "┗", "┛", "━", "┃"},
	{"ascii", "+", "+", "+", "+", "-", "|"},
}

var separators = []string{"│", "|", "┃", "·", "•", "/", "›", "»", "◆", "⋮", "space"}

// Embouts des barres : id -> gauche, droite ("none" = aucun).
var capStyles = []struct{ ID, Left, Right string }{
	{"none", "", ""}, {"brackets", "[", "]"}, {"thin", "▕", "▏"}, {"parens", "(", ")"}, {"angles", "⟨", "⟩"},
}

// capsOf résout le défaut : ASCII garde ses crochets historiques.
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
	{"blocks", "Blocs"}, {"fine", "Fin"}, {"line", "Ligne"}, {"dots", "Points"},
	{"braille", "Braille"}, {"ascii", "ASCII"}, {"text", "Texte"},
	{"squares", "Carrés"}, {"diamonds", "Losanges"}, {"segments", "Segments"}, {"dashes", "Tirets"},
	{"shade", "Ombre"}, {"slider", "Curseur"}, {"ramp", "Rampe"},
}

func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "statusline-builder.json")
}

func loadConfig() (Config, error) {
	var cfg Config
	b, err := os.ReadFile(configPath())
	if err != nil {
		return cfg, err
	}
	return cfg, json.Unmarshal(b, &cfg)
}

type rateWindow struct {
	UsedPercentage *float64 `json:"used_percentage"`
	ResetsAt       *float64 `json:"resets_at"`
}

// Sous-ensemble du JSON de session Claude Code utilisé par les widgets.
type session struct {
	Cwd   string `json:"cwd"`
	Model struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
	} `json:"model"`
	Workspace struct {
		CurrentDir string `json:"current_dir"`
	} `json:"workspace"`
	Cost struct {
		TotalCostUSD      *float64 `json:"total_cost_usd"`
		TotalLinesAdded   *int     `json:"total_lines_added"`
		TotalLinesRemoved *int     `json:"total_lines_removed"`
	} `json:"cost"`
	ContextWindow struct {
		UsedPercentage *float64 `json:"used_percentage"`
	} `json:"context_window"`
	RateLimits struct {
		FiveHour *rateWindow `json:"five_hour"`
		SevenDay *rateWindow `json:"seven_day"`
	} `json:"rate_limits"`
}

func (s session) dir() string {
	if s.Workspace.CurrentDir != "" {
		return s.Workspace.CurrentDir
	}
	return s.Cwd
}

func gitBranch(dir string) string {
	if dir == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", "-C", dir, "branch", "--show-current").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// paint colore s avec une couleur résolue par colorSpec ("" = pas de couleur).
func paint(s, spec string) string {
	if s == "" || spec == "" {
		return s
	}
	if spec[0] == '#' {
		var r, g, b int
		fmt.Sscanf(spec, "#%02x%02x%02x", &r, &g, &b)
		return fmt.Sprintf("\x1b[38;2;%d;%d;%dm%s\x1b[0m", r, g, b, s)
	}
	return "\x1b[38;5;" + spec + "m" + s + "\x1b[0m"
}

func dim(s string) string {
	if s == "" {
		return s
	}
	return "\x1b[2m" + s + "\x1b[0m"
}

// colorSpec résout une couleur de config : nom de la palette, index 256 ("178")
// ou "#rrggbb". Renvoie l'index ou le hex, "" si inconnue.
func colorSpec(name string) string {
	if len(name) == 7 && name[0] == '#' {
		return name
	}
	if n, err := strconv.Atoi(name); err == nil && n >= 0 && n < 256 {
		return name
	}
	for _, c := range colors {
		if c.Name == name {
			return strconv.Itoa(c.Index)
		}
	}
	return ""
}

// bar renvoie la partie remplie et la partie vide d'une barre de w caractères.
func bar(style string, pct float64, w int) (string, string) {
	steps := func(n int) int { return int(math.Round(pct * float64(w*n) / 100)) }
	// Styles à sous-caractères : le dernier caractère rempli est partiel.
	partial := func(levels []rune, emptyCh string) (string, string) {
		n := len(levels) - 1
		t := steps(n)
		f := t / n
		s := strings.Repeat(string(levels[n]), f)
		if r := t % n; r > 0 && f < w {
			s += string(levels[r])
			f++
		}
		return s, strings.Repeat(emptyCh, w-f)
	}
	switch style {
	case "text":
		return "", ""
	case "fine":
		return partial([]rune(" ▏▎▍▌▋▊▉█"), " ")
	case "braille":
		return partial([]rune("⣀⣄⣤⣦⣶⣷⣿"), "⣀")
	}
	f := steps(1)
	switch style {
	case "slider": // rail rempli jusqu'au curseur ●, qui occupe une cellule
		f = min(f, w-1)
		return strings.Repeat("━", f) + "●", strings.Repeat("─", w-f-1)
	case "ramp": // hauteur croissante selon la position dans la barre
		levels := []rune("▁▂▃▄▅▆▇█")
		var s strings.Builder
		for i := range f {
			s.WriteRune(levels[((i+1)*8+w-1)/w-1])
		}
		return s.String(), strings.Repeat("·", w-f)
	}
	ch, ok := map[string][2]string{
		"line": {"━", "─"}, "dots": {"●", "○"}, "ascii": {"#", "-"},
		"squares": {"■", "□"}, "diamonds": {"◆", "◇"}, "segments": {"▰", "▱"},
		"dashes": {"▬", "▭"}, "shade": {"▓", "░"},
	}[style]
	if !ok {
		ch = [2]string{"█", "░"}
	}
	return strings.Repeat(ch[0], f), strings.Repeat(ch[1], w-f)
}

func gauge(c Cell, used *float64, label string) string {
	if used == nil {
		return ""
	}
	u := math.Max(0, math.Min(100, *used))
	p := u
	if c.Mode == "left" {
		p = 100 - u
	}
	w := c.Width
	if w == 0 {
		w = 10
	}
	col := colorSpec(c.Color)
	if s, ok := schemes[c.Color]; ok {
		col = s[0]
		if u >= 80 {
			col = s[2]
		} else if u >= 50 {
			col = s[1]
		}
	}

	var parts []string
	if !c.NoLabel {
		parts = append(parts, dim(label))
	}
	fill, empty := bar(c.Bar, p, w)
	if fill+empty != "" {
		b := paint(fill, col) + dim(empty)
		for _, cp := range capStyles {
			if cp.ID == capsOf(c) {
				b = dim(cp.Left) + b + dim(cp.Right)
			}
		}
		parts = append(parts, b)
	}
	parts = append(parts, paint(fmt.Sprintf("%.0f%%", p), col))
	return strings.Join(parts, " ")
}

// modelVersion extrait la version d'un id de modèle :
// "claude-opus-5-5" -> "5.5", "claude-3-5-sonnet-20241022" -> "3.5", "claude-opus-4-1[1m]" -> "4.1".
func modelVersion(id string) string {
	id, _, _ = strings.Cut(id, "[")
	var nums []string
	for _, p := range strings.Split(id, "-") {
		if len(p) > 0 && len(p) <= 2 && strings.Trim(p, "0123456789") == "" {
			nums = append(nums, p) // les dates (8 chiffres) sont ignorées
		}
	}
	return strings.Join(nums, ".")
}

var weekdays = [...]string{"dim", "lun", "mar", "mer", "jeu", "ven", "sam"}

func humanize(d time.Duration) string {
	s := max(int(d.Seconds()), 0)
	switch {
	case s >= 86400:
		return fmt.Sprintf("%dj%02dh", s/86400, s%86400/3600)
	case s >= 3600:
		return fmt.Sprintf("%dh%02dm", s/3600, s%3600/60)
	case s >= 60:
		return fmt.Sprintf("%dm", s/60)
	}
	return fmt.Sprintf("%ds", s)
}

func reset(c Cell, w *rateWindow, label string, withDay bool) string {
	if w == nil || w.ResetsAt == nil {
		return ""
	}
	t := time.Unix(int64(*w.ResetsAt), 0)
	at := t.Format("15:04")
	if withDay {
		at = weekdays[t.Weekday()] + " " + at
	}
	return dim(label) + " " + paint(at, colorSpec(c.Color)) + " " + dim("("+humanize(time.Until(t))+")")
}

func usedOf(w *rateWindow) *float64 {
	if w == nil {
		return nil
	}
	if w.UsedPercentage == nil {
		return new(float64) // fenêtre présente sans pourcentage : 0 %
	}
	return w.UsedPercentage
}

// cellValue renvoie le texte coloré d'une cellule, "" si la donnée est absente.
func cellValue(c Cell, s session) string {
	col := colorSpec(c.Color)
	switch c.Widget {
	case "model":
		name := s.Model.DisplayName
		if name == "" {
			return ""
		}
		// N'ajoute la version que si le nom affiché n'en contient pas déjà une.
		if v := modelVersion(s.Model.ID); c.Version && v != "" && !strings.ContainsAny(name, "0123456789") {
			name += " " + v
		}
		if c.Icon {
			name = "✳ " + name
		}
		return paint(name, col)
	case "dir":
		d := s.dir()
		if d == "" {
			return ""
		}
		if !c.Full {
			return paint(filepath.Base(d), col)
		}
		if home, _ := os.UserHomeDir(); home != "" && (d == home || strings.HasPrefix(d, home+"/")) {
			d = "~" + strings.TrimPrefix(d, home)
		}
		return paint(d, col)
	case "git":
		return paint(gitBranch(s.dir()), col)
	case "userhost":
		u, err := user.Current()
		h, _ := os.Hostname()
		if err != nil || h == "" {
			return ""
		}
		h, _, _ = strings.Cut(h, ".")
		return paint(u.Username+"@"+h, col)
	case "cost":
		if s.Cost.TotalCostUSD != nil {
			return paint(fmt.Sprintf("$%.2f", *s.Cost.TotalCostUSD), col)
		}
	case "lines":
		if s.Cost.TotalLinesAdded == nil || s.Cost.TotalLinesRemoved == nil {
			return ""
		}
		add, del := "2", "1" // vert / rouge sauf couleur choisie
		if col != "" {
			add, del = col, col
		}
		return paint(fmt.Sprintf("+%d", *s.Cost.TotalLinesAdded), add) + " " +
			paint(fmt.Sprintf("−%d", *s.Cost.TotalLinesRemoved), del)
	case "context":
		return gauge(c, s.ContextWindow.UsedPercentage, "ctx")
	case "limit5h":
		return gauge(c, usedOf(s.RateLimits.FiveHour), "5h")
	case "limit7d":
		return gauge(c, usedOf(s.RateLimits.SevenDay), "7j")
	case "reset5h":
		return reset(c, s.RateLimits.FiveHour, "5h ↻", false)
	case "reset7d":
		return reset(c, s.RateLimits.SevenDay, "7j ↻", true)
	case "text":
		return paint(c.Text, col)
	}
	return ""
}

// render produit la status line. JSON invalide => champs vides, jamais d'erreur.
// Les couleurs sont des codes ANSI bruts : stdout n'est pas un TTY quand Claude
// Code capture la sortie, lipgloss les supprimerait.
func render(input []byte, cfg Config) string {
	var s session
	_ = json.Unmarshal(input, &s)
	ch, pad := cfg.Sep, " "
	switch ch {
	case "":
		ch = "│"
	case "space":
		ch = ""
	}
	if cfg.AirCols {
		pad = "   "
	}
	sep := pad + ch + pad
	var lines []string
	for i, row := range cfg.Rows {
		// Une cellule vide entre deux widgets devient un blanc (sans séparateur) de la
		// largeur de deux séparateurs, et un de plus par cellule vide supplémentaire.
		// Un widget sans donnée est simplement sauté, il ne crée pas de blanc.
		var line string
		empty := 0
		for _, c := range row {
			if c.Widget == "" {
				empty++
				continue
			}
			v := cellValue(c, s)
			if v == "" {
				continue
			}
			if line != "" {
				if empty == 0 {
					line += sep
				} else {
					line += strings.Repeat(" ", utf8.RuneCountInString(sep)*(empty+1))
				}
			}
			line += v
			empty = 0
		}
		if line == "" {
			continue // ligne sans donnée : ni la ligne ni son espace
		}
		lines = append(lines, line)
		if i < len(cfg.Gaps) && cfg.Gaps[i] {
			// Espace de largeur nulle plutôt qu'une ligne vide, comme dans l'ancien statusline-command.sh.
			lines = append(lines, "​")
		}
	}
	return strings.Join(frame(lines, cfg), "\n")
}

// frame entoure les lignes d'un contour à la largeur de la plus longue.
// Les lignes vides (espace de largeur nulle) deviennent des lignes vides du cadre.
func frame(lines []string, cfg Config) []string {
	for _, b := range borders {
		if b.ID != cfg.Border || len(lines) == 0 {
			continue
		}
		bc := colorSpec(cfg.BorderColor)
		edge := func(s string) string {
			if bc == "" {
				return dim(s)
			}
			return paint(s, bc)
		}
		w := 0
		for _, l := range lines {
			w = max(w, ansi.StringWidth(l))
		}
		out := []string{edge(b.TL + strings.Repeat(b.H, w+2) + b.TR)}
		for _, l := range lines {
			if l == "\u200b" {
				l = ""
			}
			out = append(out, edge(b.V)+" "+l+strings.Repeat(" ", w-ansi.StringWidth(l))+" "+edge(b.V))
		}
		return append(out, edge(b.BL+strings.Repeat(b.H, w+2)+b.BR))
	}
	return lines
}
