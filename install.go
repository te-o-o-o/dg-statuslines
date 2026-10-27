package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
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
