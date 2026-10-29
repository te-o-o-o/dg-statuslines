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
