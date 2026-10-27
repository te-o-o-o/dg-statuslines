package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone"
)

const (
	cellWidth = 14
	minW      = 60 // en dessous : état « terminal trop petit »
	minH      = 20
)

var tabNames = []string{"Disposition", "Style", "Installer"}

type pos struct{ r, c int }

type drag struct {
	from *pos // nil : vient de la palette
	cell Cell
}

// toastDone efface le message temporaire n, s'il n'a pas été remplacé entre-temps.
type toastDone int

type model struct {
	cfg     Config
	saved   string // config sauvegardée (JSON), pour l'indicateur « non sauvegardé »
	sample  []byte
	branch  string // branche affichée dans l'aperçu (calculée une fois au démarrage)
	tab     int
	w, h    int
	sel     *pos
	selRow  *int   // ligne sélectionnée par sa gouttière (L1, L2…)
	pop     string // liste de choix ouverte ("" = aucune)
	popIdx  int
	editing bool // saisie du texte libre de la cellule sélectionnée
	drag    *drag
	hover   string // zone survolée pendant le drag
	mx, my  int
	toast   string
	toastN  int

	// Onglet Installer, recalculé à chaque ouverture de l'onglet.
	plan           []byte
	oldCmd, newCmd string
	planErr        error
}

var (
	accent    = lipgloss.Color("212")
	edgeColor = lipgloss.Color("240")
	faint     = lipgloss.NewStyle().Faint(true)
	title     = lipgloss.NewStyle().Bold(true)
	heading   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("245"))
	keyStyle  = lipgloss.NewStyle().Bold(true)
	okStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	warnStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	errStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	button    = lipgloss.NewStyle().Padding(0, 1).Background(lipgloss.Color("237"))
	dragLabel = lipgloss.NewStyle().Padding(0, 1).Background(accent).Foreground(lipgloss.Color("0"))
	box       = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
)

func edit() error {
	cfg, err := loadConfig()
	saved := ""
	if err != nil || len(cfg.Rows) == 0 {
		cfg = Config{Rows: [][]Cell{make([]Cell, 4), make([]Cell, 4)}}
	}
	// Toutes les lignes ont le même nombre de colonnes.
	cols := 1
	for _, r := range cfg.Rows {
		cols = max(cols, len(r))
	}
	for i := range cfg.Rows {
		for len(cfg.Rows[i]) < cols {
			cfg.Rows[i] = append(cfg.Rows[i], Cell{})
		}
	}
	if err == nil {
		saved = configJSON(cfg)
	}

	cwd, _ := os.Getwd()
	now := time.Now().Unix()
	sample, _ := json.Marshal(map[string]any{
		"model":          map[string]any{"id": "claude-opus-5-5", "display_name": "Opus"},
		"workspace":      map[string]any{"current_dir": cwd},
		"cost":           map[string]any{"total_cost_usd": 0.42, "total_lines_added": 156, "total_lines_removed": 23},
		"context_window": map[string]any{"used_percentage": 23},
		"rate_limits": map[string]any{
			"five_hour": map[string]any{"used_percentage": 62.5, "resets_at": now + 2*3600 + 12*60},
			"seven_day": map[string]any{"used_percentage": 85.2, "resets_at": now + 3*86400 + 4*3600},
		},
	})

	branch := gitBranch(cwd)
	if branch == "" {
		branch = "main" // hors dépôt git : valeur d'exemple pour que le widget reste visible
	}

	zone.NewGlobal()
	_, err = tea.NewProgram(model{cfg: cfg, saved: saved, sample: sample, branch: branch}, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()
	return err
}

func configJSON(cfg Config) string {
	b, _ := json.MarshalIndent(cfg, "", "  ")
	return string(b)
}

func (m model) dirty() bool { return configJSON(m.cfg) != m.saved }

func (m model) Init() tea.Cmd { return nil }

func cellID(r, c int) string { return fmt.Sprintf("cell:%d:%d", r, c) }

func parseCell(id string) (pos, bool) {
	var p pos
	_, err := fmt.Sscanf(id, "cell:%d:%d", &p.r, &p.c)
	return p, err == nil
}

// hit renvoie l'id de la zone sous la souris ("" si aucune).
func (m model) hit(msg tea.MouseMsg) string {
	ids := []string{"tab:0", "tab:1", "tab:2", "btn:install"}
	if m.pop != "" {
		// Liste ouverte : seuls ses choix sont cliquables, le reste la ferme.
		for _, it := range m.popItems() {
			ids = append(ids, it.ID)
		}
	} else {
		for r, row := range m.cfg.Rows {
			ids = append(ids, fmt.Sprintf("row:%d", r), fmt.Sprintf("gap:%d", r))
			for c := range row {
				ids = append(ids, cellID(r, c))
			}
		}
		for _, w := range widgets {
			ids = append(ids, "pal:"+w.ID)
		}
		ids = append(ids, inspIDs...)
		for _, sp := range separators {
			ids = append(ids, "sep:"+sp)
		}
		for _, t := range themes {
			ids = append(ids, "theme:"+t.ID)
		}
		for _, b := range borders {
			ids = append(ids, "border:"+b.ID)
		}
		for _, c := range colors {
			ids = append(ids, "bcolor:"+c.Name)
		}
		ids = append(ids, "border:", "bcolor:")
		ids = append(ids, "btn:+row", "btn:-row", "btn:+col", "btn:-col", "btn:airrows", "btn:aircols", "palette")
	}
	for _, id := range ids {
		if zone.Get(id).InBounds(msg) {
			return id
		}
	}
	return ""
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
	case toastDone:
		if int(msg) == m.toastN {
			m.toast = ""
		}
	case tea.KeyMsg:
		return m.key(msg)
	case tea.MouseMsg:
		m.mx, m.my = msg.X, msg.Y
		hit := m.hit(msg)
		switch msg.Action {
		case tea.MouseActionPress:
			if msg.Button == tea.MouseButtonLeft {
				return m.press(hit)
			}
		case tea.MouseActionMotion:
			if m.drag != nil {
				m.hover = hit
			}
		case tea.MouseActionRelease:
			if m.drag != nil {
				m.drop(hit)
			}
			m.drag, m.hover = nil, ""
		}
	}
	return m, nil
}

// notify affiche un message temporaire (succès : 3 s ; erreur : jusqu'au suivant).
func (m *model) notify(msg string, err bool) tea.Cmd {
	m.toastN++
	if err {
		m.toast = errStyle.Render("✗ ") + msg
		return nil
	}
	m.toast = okStyle.Render("✓ ") + msg
	n := m.toastN
	return tea.Tick(3*time.Second, func(time.Time) tea.Msg { return toastDone(n) })
}

func (m *model) setTab(t int) {
	m.tab, m.editing = t, false
	if t == 2 {
		m.plan, m.oldCmd, m.newCmd, m.planErr = planInstall()
	}
}

func (m *model) save() tea.Cmd {
	s := configJSON(m.cfg)
	if err := os.WriteFile(configPath(), []byte(s+"\n"), 0o644); err != nil {
		return m.notify("Sauvegarde impossible : "+err.Error(), true)
	}
	m.saved = s
	return m.notify("Sauvegardé", false)
}

func (m *model) install() tea.Cmd {
	if m.planErr != nil {
		return m.notify("settings.json illisible : "+m.planErr.Error(), true)
	}
	if cmd := m.save(); m.dirty() {
		return cmd // la sauvegarde de la config a échoué
	}
	if err := applyInstall(m.plan); err != nil {
		return m.notify("Installation impossible : "+err.Error(), true)
	}
	m.setTab(2) // relit settings.json pour mettre le statut à jour
	return m.notify("Installé dans Claude Code", false)
}

func (m model) press(hit string) (tea.Model, tea.Cmd) {
	m.editing = false
	if m.pop != "" {
		if hit != "" && !strings.HasPrefix(hit, "tab:") {
			m.apply(hit)
		}
		m.pop = ""
		return m, nil
	}
	if p, ok := parseCell(hit); ok {
		m.selRow = nil
		if c := m.cfg.Rows[p.r][p.c]; c.Widget != "" {
			m.drag = &drag{from: &p, cell: c}
		} else {
			m.sel = &p
		}
		return m, nil
	}
	switch {
	case strings.HasPrefix(hit, "row:"):
		r, _ := strconv.Atoi(strings.TrimPrefix(hit, "row:"))
		m.sel, m.selRow = nil, &r
	case hit == "insp:text":
		m.editing = true
	case strings.HasPrefix(hit, "insp:"):
		m.openPop(strings.TrimPrefix(hit, "insp:"))
	case hit == "btn:remove" && m.sel != nil:
		m.cfg.Rows[m.sel.r][m.sel.c] = Cell{}
	case strings.HasPrefix(hit, "color:"), strings.HasPrefix(hit, "opt:"):
		m.apply(hit)
	case strings.HasPrefix(hit, "tab:"):
		t, _ := strconv.Atoi(strings.TrimPrefix(hit, "tab:"))
		m.setTab(t)
	case hit == "btn:install":
		return m, m.install()
	case strings.HasPrefix(hit, "pal:"):
		m.drag = &drag{cell: Cell{Widget: strings.TrimPrefix(hit, "pal:")}}
	case strings.HasPrefix(hit, "gap:"):
		r, _ := strconv.Atoi(strings.TrimPrefix(hit, "gap:"))
		m.setGap(r, !m.gap(r))
	case hit == "btn:airrows":
		on := !m.allGaps()
		for r := range len(m.cfg.Rows) - 1 {
			m.setGap(r, on)
		}
	case hit == "btn:aircols":
		m.cfg.AirCols = !m.cfg.AirCols
	case strings.HasPrefix(hit, "theme:"):
		m.applyTheme(strings.TrimPrefix(hit, "theme:"))
	case strings.HasPrefix(hit, "sep:"):
		m.cfg.Sep = strings.TrimPrefix(hit, "sep:")
	case strings.HasPrefix(hit, "border:"):
		m.cfg.Border = strings.TrimPrefix(hit, "border:")
	case strings.HasPrefix(hit, "bcolor:"):
		m.cfg.BorderColor = strings.TrimPrefix(hit, "bcolor:")
	case hit == "btn:+row":
		m.cfg.Rows = append(m.cfg.Rows, make([]Cell, len(m.cfg.Rows[0])))
	case hit == "btn:-row" && len(m.cfg.Rows) > 1:
		m.cfg.Rows = m.cfg.Rows[:len(m.cfg.Rows)-1]
		m.cfg.Gaps = m.cfg.Gaps[:min(len(m.cfg.Gaps), len(m.cfg.Rows))]
	case hit == "btn:+col" && len(m.cfg.Rows[0]) < 8:
		for i := range m.cfg.Rows {
			m.cfg.Rows[i] = append(m.cfg.Rows[i], Cell{})
		}
	case hit == "btn:-col" && len(m.cfg.Rows[0]) > 1:
		for i := range m.cfg.Rows {
			m.cfg.Rows[i] = m.cfg.Rows[i][:len(m.cfg.Rows[i])-1]
		}
	}
	if m.sel != nil && (m.sel.r >= len(m.cfg.Rows) || m.sel.c >= len(m.cfg.Rows[0])) {
		m.sel = nil
	}
	if m.selRow != nil && *m.selRow >= len(m.cfg.Rows) {
		m.selRow = nil
	}
	return m, nil
}

// apply applique un choix sur la cellule sélectionnée (liste ou inspecteur).
func (m *model) apply(id string) {
	if m.sel == nil {
		return
	}
	c := &m.cfg.Rows[m.sel.r][m.sel.c]
	switch {
	case strings.HasPrefix(id, "place:"):
		*c = Cell{Widget: strings.TrimPrefix(id, "place:")}
		if th, ok := themeByID(m.cfg.Theme); ok {
			c.Color = th.color(c.Widget)
		}
		m.editing = c.Widget == "text"
	case strings.HasPrefix(id, "color:"):
		c.Color = strings.TrimPrefix(id, "color:")
	case strings.HasPrefix(id, "opt:"):
		k, v, _ := strings.Cut(strings.TrimPrefix(id, "opt:"), ":")
		switch k {
		case "bar":
			c.Bar = v
		case "mode":
			c.Mode = v
		case "width":
			c.Width, _ = strconv.Atoi(v)
		case "label":
			c.NoLabel = v == "off"
		case "caps":
			c.Caps = v
		case "version":
			c.Version = v == "yes"
		case "icon":
			c.Icon = v == "yes"
		case "full":
			c.Full = v == "yes"
		}
	}
}

// moveSel déplace la cellule sélectionnée au clavier.
func (m *model) moveSel(dr, dc int) {
	if m.sel == nil {
		m.sel, m.selRow = &pos{0, 0}, nil
		return
	}
	r := min(max(m.sel.r+dr, 0), len(m.cfg.Rows)-1)
	c := min(max(m.sel.c+dc, 0), len(m.cfg.Rows[0])-1)
	m.sel = &pos{r, c}
}

func (m *model) drop(hit string) {
	d := m.drag
	if t, ok := parseCell(hit); ok {
		if d.from == nil {
			m.cfg.Rows[t.r][t.c] = d.cell // palette -> cellule (remplace)
			if th, ok := themeByID(m.cfg.Theme); ok {
				m.cfg.Rows[t.r][t.c].Color = th.color(d.cell.Widget)
			}
		} else if *d.from != t {
			// cellule -> cellule : déplace, ou échange si la cible est occupée
			rows := m.cfg.Rows
			rows[t.r][t.c], rows[d.from.r][d.from.c] = rows[d.from.r][d.from.c], rows[t.r][t.c]
		}
		m.sel = &t
		m.editing = m.cfg.Rows[t.r][t.c].Widget == "text"
		return
	}
	if (hit == "palette" || strings.HasPrefix(hit, "pal:")) && d.from != nil {
		m.cfg.Rows[d.from.r][d.from.c] = Cell{} // cellule -> palette : retire
		m.sel = nil
	}
}

func (m model) key(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}
	if m.editing && m.sel != nil {
		c := &m.cfg.Rows[m.sel.r][m.sel.c]
		switch msg.Type {
		case tea.KeyEnter, tea.KeyEsc:
			m.editing = false
		case tea.KeyBackspace:
			if r := []rune(c.Text); len(r) > 0 {
				c.Text = string(r[:len(r)-1])
			}
		case tea.KeyRunes, tea.KeySpace:
			c.Text += string(msg.Runes)
		}
		return m, nil
	}
	if m.pop != "" {
		items := m.popItems()
		step := map[string]int{"up": -1, "down": 1, "left": -1, "right": 1}
		if m.pop == "color" {
			step["up"], step["down"] = -8, 8 // rangées de 8 carrés
		} else if rows := popRows(len(items)); rows < len(items) {
			step["left"], step["right"] = -rows, rows // deux colonnes
		}
		switch k := msg.String(); k {
		case "up", "down", "left", "right":
			if n := len(items); n > 0 {
				m.popIdx = ((m.popIdx+step[k])%n + n) % n
			}
		case "enter", " ":
			if m.popIdx < len(items) {
				m.apply(items[m.popIdx].ID)
			}
			m.pop = ""
		case "esc", "q":
			m.pop = ""
		}
		return m, nil
	}
	if m.tab == 0 {
		switch msg.String() {
		case "up":
			m.moveSel(-1, 0)
			return m, nil
		case "down":
			m.moveSel(1, 0)
			return m, nil
		case "left":
			m.moveSel(0, -1)
			return m, nil
		case "right":
			m.moveSel(0, 1)
			return m, nil
		case "a", "enter":
			if c := m.selCell(); c != nil && (c.Widget == "" || msg.String() == "a") {
				m.openPop("widget")
			} else if c != nil && c.Widget == "text" {
				m.editing = true
			}
			return m, nil
		case "x":
			if m.sel != nil {
				m.cfg.Rows[m.sel.r][m.sel.c] = Cell{}
			}
			return m, nil
		case "c":
			if c := m.selCell(); c != nil && c.Widget != "" {
				m.openPop("color")
			}
			return m, nil
		case "b":
			if c := m.selCell(); c != nil && isGauge(c.Widget) {
				m.openPop("bar")
			}
			return m, nil
		}
	}
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "1", "2", "3":
		m.setTab(int(msg.Runes[0] - '1'))
	case "tab":
		m.setTab((m.tab + 1) % len(tabNames))
	case "shift+tab":
		m.setTab((m.tab + len(tabNames) - 1) % len(tabNames))
	case "esc":
		m.sel, m.selRow = nil, nil
	case "s":
		return m, m.save()
	case "i":
		m.setTab(2)
	case "enter":
		if m.tab == 2 {
			return m, m.install()
		}
	}
	return m, nil
}

func label(id string) string {
	for _, w := range widgets {
		if w.ID == id {
			return w.Label
		}
	}
	return id
}

func colorOf(name string) lipgloss.Style {
	if spec := colorSpec(name); spec != "" {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(spec))
	}
	return lipgloss.NewStyle()
}

func toggleStyle(on bool) lipgloss.Style {
	if on {
		return button.Background(accent).Foreground(lipgloss.Color("0"))
	}
	return button
}

func toggle(id, label string, on bool) string {
	return zone.Mark(id, toggleStyle(on).Render(label))
}

func themeByID(id string) (theme, bool) {
	for _, t := range themes {
		if t.ID == id {
			return t, true
		}
	}
	return theme{}, false
}

// applyTheme recolore toutes les cellules et le contour (remplace les couleurs choisies à la main).
func (m *model) applyTheme(id string) {
	t, ok := themeByID(id)
	if !ok {
		return
	}
	m.cfg.Theme, m.cfg.BorderColor = id, t.Border
	for r := range m.cfg.Rows {
		for c := range m.cfg.Rows[r] {
			if w := m.cfg.Rows[r][c].Widget; w != "" {
				m.cfg.Rows[r][c].Color = t.color(w)
			}
		}
	}
}

func (m model) gap(r int) bool { return r < len(m.cfg.Gaps) && m.cfg.Gaps[r] }

func (m *model) setGap(r int, on bool) {
	for len(m.cfg.Gaps) <= r {
		m.cfg.Gaps = append(m.cfg.Gaps, false)
	}
	m.cfg.Gaps[r] = on
}

// allGaps : une ligne vide entre chaque paire de lignes.
func (m model) allGaps() bool {
	for r := range len(m.cfg.Rows) - 1 {
		if !m.gap(r) {
			return false
		}
	}
	return len(m.cfg.Rows) > 1
}

// width : largeur utile, 80 tant que le terminal n'a pas donné sa taille.
func (m model) width() int {
	if m.w == 0 {
		return 80
	}
	return m.w
}

// titledBox dessine un cadre gris avec un titre dans le bord haut, à la largeur w.
func titledBox(name, content string, w int) string {
	edge := lipgloss.NewStyle().Foreground(edgeColor)
	inner := w - 4
	top := "╭─ " + name + " " + strings.Repeat("─", max(0, w-len([]rune(name))-5)) + "╮"
	lines := []string{edge.Render(top)}
	for _, l := range strings.Split(content, "\n") {
		l = ansi.Truncate(l, inner, "…")
		lines = append(lines, edge.Render("│")+" "+l+strings.Repeat(" ", max(0, inner-ansi.StringWidth(l)))+" "+edge.Render("│"))
	}
	lines = append(lines, edge.Render("╰"+strings.Repeat("─", w-2)+"╯"))
	return strings.Join(lines, "\n")
}

func (m model) header() string {
	left := " " + title.Render("slb") + "   "
	for i, n := range tabNames {
		st := faint
		if i == m.tab {
			st = lipgloss.NewStyle().Bold(true).Underline(true).Foreground(accent)
		}
		left += zone.Mark(fmt.Sprintf("tab:%d", i), st.Render(n)) + "   "
	}
	right := m.toast
	if right == "" {
		if m.dirty() {
			right = warnStyle.Render("● non sauvegardé")
		} else {
			right = faint.Render("✓ sauvegardé")
		}
	}
	gap := max(1, m.width()-ansi.StringWidth(left)-ansi.StringWidth(right)-1)
	return left + strings.Repeat(" ", gap) + right
}

func (m model) previewBox() string {
	// Liste ouverte : l'aperçu montre le choix sous le focus, sans le valider.
	if items := m.popItems(); m.pop != "" && m.popIdx < len(items) {
		trial := m
		trial.cfg = cloneCfg(m.cfg)
		trial.apply(items[m.popIdx].ID)
		m.cfg = trial.cfg
	}
	// Les cellules git deviennent du texte fixe : pas de sous-processus git à chaque frame.
	pcfg := m.cfg // mêmes réglages globaux, lignes recopiées ci-dessous
	pcfg.Rows = nil
	for _, row := range m.cfg.Rows {
		prow := append([]Cell(nil), row...)
		for i, c := range prow {
			if c.Widget == "git" {
				prow[i] = Cell{Widget: "text", Text: m.branch, Color: c.Color}
			}
		}
		pcfg.Rows = append(pcfg.Rows, prow)
	}
	preview := render(m.sample, pcfg)
	if preview == "" {
		preview = faint.Render("Glisse un widget dans la grille pour commencer.")
	}
	return titledBox("Aperçu", preview, m.width()-2)
}

// hints affiche une ligne d'aide « touche action · touche action ».
func hints(pairs ...string) string {
	var parts []string
	for i := 0; i+1 < len(pairs); i += 2 {
		parts = append(parts, keyStyle.Render(pairs[i])+" "+faint.Render(pairs[i+1]))
	}
	return " " + strings.Join(parts, faint.Render(" · "))
}

func (m model) footer() string {
	switch {
	case m.editing:
		return hints("entrée", "terminer la saisie", "⌫", "effacer")
	case m.pop != "":
		return hints("↑↓←→", "naviguer", "entrée", "choisir", "esc", "fermer")
	case m.tab == 0 && m.sel != nil:
		return hints("←↑↓→", "cellule", "a", "ajouter", "x", "retirer", "c", "couleur", "esc", "retour", "s", "sauver")
	case m.tab == 0:
		return hints("glisser", "placer", "←↑↓→", "cellule", "1-3", "onglet", "s", "sauver", "q", "quitter")
	case m.tab == 1:
		return hints("clic", "choisir", "1-3", "onglet", "s", "sauver", "q", "quitter")
	}
	return hints("entrée", "sauvegarder et installer", "1-3", "onglet", "q", "quitter")
}

// labelW : largeur de la colonne d'intitulés, commune à tous les réglages.
const labelW = 12

// field aligne un réglage : intitulé dans une colonne fixe, contrôles à droite.
func field(name, content string) string {
	return lipgloss.JoinHorizontal(lipgloss.Top, faint.Width(labelW).Render(name), content)
}

// swatch : une case du quadrillage de couleurs.
type swatch struct {
	ID     string   // valeur enregistrée ("" = couleur par défaut)
	Colors []string // couleurs affichées côte à côte ; vide = case texte (Label)
	Label  string
}

// colorItems : les 24 couleurs, puis le choix « par défaut » (dernier, pour que
// la navigation au clavier tombe juste sur les rangées de 8).
func colorItems(defaultLabel string) []swatch {
	var items []swatch
	for _, c := range colors {
		items = append(items, swatch{ID: c.Name, Colors: []string{c.Name}})
	}
	return append(items, swatch{ID: "", Label: defaultLabel})
}

// swatchGrid dessine des carrés de couleur cliquables (zone prefix+ID), perRow par rangée,
// une ligne vide entre les rangées. Un carré fait 2 caractères sur 1 ligne, soit un carré
// à l'écran. Les choix sans couleur (« auto ») sont des boutons au bout de la première rangée.
func swatchGrid(prefix, cur, focus string, items []swatch, perRow int) string {
	var squares, chips []string
	for _, it := range items {
		var cell string
		switch {
		case len(it.Colors) == 0:
			cell = toggleStyle(it.ID == cur).Underline(it.ID == focus && focus != "").Render(it.Label)
			chips = append(chips, zone.Mark(prefix+it.ID, cell))
			continue
		case it.ID == focus && focus != "":
			cell = swatchCell(it, "◆")
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

// swatchCell : 2 caractères de fond par couleur ; mark (✓ choisi, ◆ focus clavier) sur la couleur du milieu.
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

// isLight dit si une couleur (index 256 ou #rrggbb) est claire, pour choisir
// une coche noire ou blanche lisible dessus. Les 16 couleurs de base dépendent du
// thème du terminal : valeurs xterm par défaut.
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
	// Thèmes : 3 par ligne, noms à largeur fixe pour que les colonnes tombent juste.
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
			label = "espace"
		}
		seps = append(seps, toggle("sep:"+sp, label, m.cfg.Sep == sp || m.cfg.Sep == "" && sp == "│"))
	}

	borderChips := []string{toggle("border:", "aucun", m.cfg.Border == "")}
	for _, b := range borders {
		borderChips = append(borderChips, toggle("border:"+b.ID, b.TL+b.H+b.TR, m.cfg.Border == b.ID))
	}

	parts := []string{
		field("Thème", strings.Join(themeRows, "\n\n")), "",
		field("Séparateur", strings.Join(seps, " ")), "",
		field("Contour", strings.Join(borderChips, " ")),
	}
	if m.cfg.Border != "" {
		parts = append(parts, field("", swatchGrid("bcolor:", m.cfg.BorderColor, "", colorItems("gris"), 8)))
	}
	parts = append(parts, "", field("Espacement",
		toggle("btn:airrows", "ligne vide entre les lignes", m.allGaps())+" "+
			toggle("btn:aircols", "colonnes aérées", m.cfg.AirCols)))
	return strings.Join(parts, "\n")
}

func (m model) installTab() string {
	if m.planErr != nil {
		return heading.Render("INSTALLATION") + "\n\n" +
			errStyle.Render("✗ ") + "Impossible de lire ~/.claude/settings.json : " + m.planErr.Error() + "\n" +
			faint.Render("Corrige le fichier, puis rouvre cet onglet.")
	}
	status := warnStyle.Render("○ ") + "slb n'est pas encore la status line de Claude Code"
	if m.oldCmd == m.newCmd {
		status = okStyle.Render("✓ ") + "slb est la status line de Claude Code"
	}
	cfgState := faint.Render("  ✓ sauvegardée")
	if m.dirty() {
		cfgState = warnStyle.Render("  ● non sauvegardée")
	}
	lines := []string{
		heading.Render("INSTALLATION"), "",
		faint.Render("Statut      ") + status,
		faint.Render("Config      ") + "~/.claude/statusline-builder.json" + cfgState,
		"",
	}
	if m.oldCmd != m.newCmd {
		lines = append(lines,
			faint.Render("Changement prévu dans ~/.claude/settings.json"),
			"  "+errStyle.Render("- "+m.oldCmd),
			"  "+okStyle.Render("+ "+m.newCmd),
			"",
		)
	}
	lines = append(lines,
		zone.Mark("btn:install", button.Background(accent).Foreground(lipgloss.Color("0")).Bold(true).Render("Sauvegarder et installer")),
		"",
		faint.Render("Une copie de settings.json est gardée dans settings.json.bak avant chaque modification."),
	)
	return strings.Join(lines, "\n")
}

func (m model) View() string {
	if m.w > 0 && (m.w < minW || m.h < minH) {
		return fmt.Sprintf("\n  Terminal trop petit : %d×%d minimum (actuellement %d×%d).\n  Agrandis la fenêtre, ou %s pour quitter.",
			minW, minH, m.w, m.h, keyStyle.Render("q"))
	}

	var body string
	switch m.tab {
	case 0:
		body = m.layoutTab()
	case 1:
		body = m.styleTab()
	default:
		body = m.installTab()
	}
	// Retrait de 2 colonnes pour aligner le contenu sur l'intérieur de l'aperçu.
	body = lipgloss.NewStyle().PaddingLeft(2).Render(body)

	preview := m.previewBox()
	content := strings.Join([]string{m.header(), "", preview, "", body}, "\n")
	if m.pop != "" {
		// Liste ouverte : sous l'aperçu (qui reste lisible pour voir l'essai), le reste
		// s'atténue (ses zones disparaissent avec ansi.Strip) et la liste se pose dessus.
		// Un seul zone.Scan : chaque Scan efface les zones du précédent.
		pv := m.popView()
		top := 2 + lipgloss.Height(preview) + 1
		ls := strings.Split(content, "\n")
		for i := top - 1; i < len(ls); i++ {
			ls[i] = faint.Render(ansi.Strip(ls[i]))
		}
		for len(ls) < top+lipgloss.Height(pv) {
			ls = append(ls, "")
		}
		content = overlayBlock(strings.Join(ls, "\n"), pv, max(0, (m.width()-lipgloss.Width(pv))/2), top)
	}
	view := zone.Scan(content)

	// Hauteur : l'aide reste en bas ; ce qui dépasse est coupé, avec un indicateur.
	lines := strings.Split(view, "\n")
	if m.h > 0 {
		room := m.h - 1
		if len(lines) > room {
			lines = append(lines[:room-1], faint.Render("  ↓ suite masquée, agrandis le terminal"))
		}
		for len(lines) < room {
			lines = append(lines, "")
		}
	}
	lines = append(lines, m.footer())
	for i, l := range lines {
		lines[i] = ansi.Truncate(l, m.width(), "")
	}
	view = strings.Join(lines, "\n")

	if m.drag != nil {
		view = overlay(view, dragLabel.Render("⠿ "+label(m.drag.cell.Widget)), m.mx+2, m.my)
	}
	return view
}

// overlay pose fg sur la ligne y de bg à partir de la colonne x (label qui suit le curseur).
func overlay(bg, fg string, x, y int) string {
	lines := strings.Split(bg, "\n")
	if y < 0 || y >= len(lines) {
		return bg
	}
	l := lines[y]
	if w := ansi.StringWidth(l); w < x {
		l += strings.Repeat(" ", x-w)
	}
	lines[y] = ansi.Truncate(l, x, "") + "\x1b[0m" + fg + ansi.TruncateLeft(l, x+ansi.StringWidth(fg), "")
	return strings.Join(lines, "\n")
}
