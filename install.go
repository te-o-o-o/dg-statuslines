package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"
)

func settingsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "settings.json")
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

// planInstall computes the new settings.json and returns the old and new
// status line commands. Only statusLine changes; other keys are copied as is.
func planInstall() (newContent []byte, oldCmd, newCmd string, err error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, "", "", err
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return nil, "", "", err
	}
	// Forward slashes, as Claude Code's docs ask on Windows; quotes when the path
	// has spaces, since the command goes through a shell.
	exe = filepath.ToSlash(exe)
	if strings.ContainsRune(exe, ' ') {
		exe = `"` + exe + `"`
	}
	newCmd = exe + " render"
	line, _ := json.Marshal(struct {
		Type    string `json:"type"`
		Command string `json:"command"`
	}{"command", newCmd})

	old, err := os.ReadFile(settingsPath())
	if err != nil && !os.IsNotExist(err) {
		return nil, "", "", err
	}
	var pairs []kv
	if len(bytes.TrimSpace(old)) > 0 {
		if pairs, err = parseObject(old); err != nil {
			return nil, "", "", err
		}
	}

	oldCmd = "(none)"
	found := false
	for i, p := range pairs {
		if p.Key == "statusLine" {
			var sl struct{ Command string }
			if json.Unmarshal(p.Value, &sl) == nil && sl.Command != "" {
				oldCmd = sl.Command
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
	for i, p := range pairs {
		if i > 0 {
			buf.WriteByte(',')
		}
		k, _ := json.Marshal(p.Key)
		buf.Write(k)
		buf.WriteByte(':')
		buf.Write(p.Value)
	}
	buf.WriteByte('}')
	newContent = []byte(indent(buf.Bytes()) + "\n")

	return newContent, oldCmd, newCmd, nil
}

func applyInstall(newContent []byte) error {
	p := settingsPath()
	if old, err := os.ReadFile(p); err == nil {
		if err := os.WriteFile(p+".bak", old, 0o644); err != nil {
			return err
		}
	} else if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, newContent, 0o644)
}

func (m *model) install() tea.Cmd {
	if m.planErr != nil {
		return m.notify("settings.json unreadable: "+m.planErr.Error(), true)
	}
	if cmd := m.save(); m.dirty() {
		return cmd // saving the config failed
	}
	if err := applyInstall(m.plan); err != nil {
		return m.notify("Cannot install: "+err.Error(), true)
	}
	m.setTab(2) // re-read settings.json to refresh the status
	return m.notify("Installed in Claude Code", false)
}

func (m model) installTab() string {
	if m.planErr != nil {
		return m.titled(heading.Render("INSTALLATION")) + "\n\n" +
			errStyle.Render("✗ ") + "Cannot read ~/.claude/settings.json: " + m.planErr.Error() + "\n" +
			faint.Render("Fix the file, then reopen this tab.")
	}
	status := warnStyle.Render("○ ") + "dgsl is not yet the Claude Code status line"
	if m.oldCmd == m.newCmd {
		status = okStyle.Render("✓ ") + "dgsl is the Claude Code status line"
	}
	cfgState := faint.Render("  ✓ saved")
	if m.dirty() {
		cfgState = warnStyle.Render("  ● unsaved")
	}
	lines := []string{
		m.titled(heading.Render("INSTALLATION")), "",
		faint.Render("Status      ") + status,
		faint.Render("Config      ") + "~/.claude/dg-statuslines.json" + cfgState,
		"",
	}
	if m.oldCmd != m.newCmd {
		lines = append(lines,
			faint.Render("Planned change in ~/.claude/settings.json"),
			"  "+errStyle.Render("- "+m.oldCmd),
			"  "+okStyle.Render("+ "+m.newCmd),
			"",
		)
	}
	lines = append(lines,
		zone.Mark("btn:install", button.Background(accent).Foreground(onAccent).Bold(true).Render("Save and install")),
		"",
		faint.Render("A copy of settings.json is kept in settings.json.bak before each change."),
	)
	return strings.Join(lines, "\n")
}
