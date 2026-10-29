package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// profile: a Claude Code config directory and the change dgsl would make to its settings.json.
type profile struct {
	dir            string
	plan           []byte
	oldCmd, newCmd string
	err            error
}

// claudeDir: the config directory of the Claude Code profile dgsl runs under.
func claudeDir() string {
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return filepath.Clean(d)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude")
}

// profileDirs finds every Claude Code config directory, whatever its name: the default
// ~/.claude, the current one, and any directory up to two levels under home holding a
// .claude.json (Claude Code writes one at the root of each CLAUDE_CONFIG_DIR).
func profileDirs() []string {
	home, _ := os.UserHomeDir()
	dirs := []string{filepath.Join(home, ".claude"), claudeDir()}
	for _, pat := range []string{"*/.claude.json", "*/*/.claude.json"} {
		found, _ := filepath.Glob(filepath.Join(home, pat))
		for _, f := range found {
			dirs = append(dirs, filepath.Dir(f))
		}
	}
	var out []string
	for _, d := range dirs {
		if st, err := os.Stat(d); err == nil && st.IsDir() && !slices.Contains(out, d) {
			out = append(out, d)
		}
	}
	return out
}

type kv struct {
	Key   string
	Value json.RawMessage
}

// parseObject reads the top-level keys in order,
// so settings.json can be rewritten without reordering it.
func parseObject(b []byte) ([]kv, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return nil, errors.New("settings.json is not a JSON object")
	}
	var out []kv
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return nil, err
		}
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, err
		}
		out = append(out, kv{t.(string), v})
	}
	return out, nil
}

func indent(v []byte) string {
	var buf bytes.Buffer
	if json.Indent(&buf, v, "", "  ") != nil {
		return string(v)
	}
	return buf.String()
}

// planInstall computes the new settings.json of the profile in dir, with its old and
// new status line commands. Only statusLine changes; other keys are copied as is.
func planInstall(dir string) (p profile) {
	p.dir = dir
	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	if err != nil {
		p.err = err
		return p
	}
	// Forward slashes, as Claude Code's docs ask on Windows; quotes when the path
	// has spaces, since the command goes through a shell.
	exe = filepath.ToSlash(exe)
	if strings.ContainsRune(exe, ' ') {
		exe = `"` + exe + `"`
	}
	p.newCmd = exe + " render"
	line, _ := json.Marshal(struct {
		Type    string `json:"type"`
		Command string `json:"command"`
	}{"command", p.newCmd})

	old, err := os.ReadFile(filepath.Join(dir, "settings.json"))
	if err != nil && !os.IsNotExist(err) {
		p.err = err
		return p
	}
	var pairs []kv
	if len(bytes.TrimSpace(old)) > 0 {
		if pairs, err = parseObject(old); err != nil {
			p.err = err
			return p
		}
	}

	p.oldCmd = "(none)"
	found := false
	for i, e := range pairs {
		if e.Key == "statusLine" {
			var sl struct{ Command string }
			if json.Unmarshal(e.Value, &sl) == nil && sl.Command != "" {
				p.oldCmd = sl.Command
			}
			pairs[i].Value = line
			found = true
		}
	}
	if !found {
		pairs = append(pairs, kv{"statusLine", line})
	}

	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, e := range pairs {
		if i > 0 {
			buf.WriteByte(',')
		}
		k, _ := json.Marshal(e.Key)
		buf.Write(k)
		buf.WriteByte(':')
		buf.Write(e.Value)
	}
	buf.WriteByte('}')
	p.plan = []byte(indent(buf.Bytes()) + "\n")
	return p
}

func applyInstall(dir string, newContent []byte) error {
	p := filepath.Join(dir, "settings.json")
	if old, err := os.ReadFile(p); err == nil {
		if err := os.WriteFile(p+".bak", old, 0o644); err != nil {
			return err
		}
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, newContent, 0o644)
}

// install saves the config and makes dgsl the status line of profile i.
func (m *model) install(i int) tea.Cmd {
	if i < 0 || i >= len(m.profiles) {
		return nil
	}
	p := m.profiles[i]
	if p.err != nil {
		return m.notify(tilde(p.dir)+"/settings.json unreadable: "+p.err.Error(), true)
	}
	if cmd := m.save(); m.dirty() {
		return cmd // saving the config failed
	}
	if err := applyInstall(p.dir, p.plan); err != nil {
		return m.notify("Cannot install: "+err.Error(), true)
	}
	m.setTab(2) // re-read the settings to refresh the status
	return m.notify("Installed in "+tilde(p.dir), false)
}

// currentProfile: the index of the profile dgsl runs under, -1 if not found.
func (m model) currentProfile() int {
	return slices.IndexFunc(m.profiles, func(p profile) bool { return p.dir == claudeDir() })
}

func (m model) installTab() string {
	cfgState := faint.Render("  ✓ saved")
	if m.dirty() {
		cfgState = warnStyle.Render("  ● unsaved")
	}
	lines := []string{
		m.titled(heading.Render("INSTALLATION")), "",
		faint.Render("Config      ") + "~/.claude/dg-statuslines.json" + cfgState + faint.Render("  shared by every profile"),
		"",
		m.titled(heading.Render("PROFILES") + faint.Render(fmt.Sprintf("  %d found", len(m.profiles)))),
		faint.Render("Every Claude Code config directory: ~/.claude and each CLAUDE_CONFIG_DIR."),
		"",
	}
	dirW := 0
	for _, p := range m.profiles {
		dirW = max(dirW, len(tilde(p.dir)))
	}
	cur := m.currentProfile()
	for i, p := range m.profiles {
		name := fmt.Sprintf("%-*s", dirW, tilde(p.dir))
		if i == cur {
			name += faint.Render("  this terminal")
		} else {
			name += "               "
		}
		var line string
		switch {
		case p.err != nil:
			line = errStyle.Render("✗ ") + name + "  " + errStyle.Render("settings.json unreadable: "+p.err.Error())
		case p.oldCmd == p.newCmd:
			line = okStyle.Render("✓ ") + name + "  " + faint.Render("dgsl is the status line")
		default:
			btn := mark(fmt.Sprintf("btn:install:%d", i), button.Background(accent).Foreground(onAccent).Bold(true).Render("Install"))
			now := ansi.Truncate(p.oldCmd, max(10, m.width()-dirW-40), "…")
			line = warnStyle.Render("○ ") + name + "  " + btn + "  " + faint.Render("now: "+now)
		}
		lines = append(lines, line)
	}
	if len(m.profiles) > 0 {
		lines = append(lines, "", faint.Render("Installs: "+m.profiles[0].newCmd))
	}
	lines = append(lines,
		faint.Render("A copy of settings.json is kept in settings.json.bak before each change."),
	)
	return strings.Join(lines, "\n")
}
