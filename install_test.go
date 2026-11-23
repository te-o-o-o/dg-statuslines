package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestProfileDirs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(home, "elsewhere"))
	for _, d := range []string{".claude", "elsewhere", ".work", "conf/claude-x", "plain"} {
		os.MkdirAll(filepath.Join(home, d), 0o755)
	}
	for _, f := range []string{".work/.claude.json", "conf/claude-x/.claude.json"} {
		os.WriteFile(filepath.Join(home, f), []byte("{}"), 0o644)
	}
	got := profileDirs()
	want := []string{".claude", "elsewhere", ".work", "conf/claude-x"}
	for i := range want {
		want[i] = filepath.Join(home, want[i])
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestUninstallRestoresPrevious(t *testing.T) {
	dir := t.TempDir()
	orig := `{"theme":"dark","statusLine":{"type":"command","command":"old.sh"}}`
	os.WriteFile(filepath.Join(dir, "settings.json"), []byte(orig), 0o644)
	if err := applyInstall(dir, planInstall(dir).plan); err != nil {
		t.Fatal(err)
	}
	p := planInstall(dir)
	if p.oldCmd != p.newCmd {
		t.Fatalf("not installed: %q", p.oldCmd)
	}
	if err := applyInstall(dir, p.undo); err != nil {
		t.Fatal(err)
	}
	if got := planInstall(dir).oldCmd; got != "old.sh" {
		t.Fatalf("after remove: %q, want old.sh", got)
	}
	// Without a previous status line, remove drops the key.
	os.WriteFile(filepath.Join(dir, "settings.json.bak"), []byte(`{"theme":"dark"}`), 0o644)
	if string(planInstall(dir).undo) != "{\n  \"theme\": \"dark\"\n}\n" {
		t.Fatalf("undo = %s", planInstall(dir).undo)
	}
}
