package main

// Builder appearance: the colours of the interface itself (not of the status line).

import "github.com/charmbracelet/lipgloss"

type uiTheme struct {
	ID, Label                                                                        string
	Accent, OnAccent, Edge, Rule, Heading, Group, Faint, Button, Random, RandomHover string
}

var uiThemes = []uiTheme{
	{"ocean", "Ocean", "#5fd7ff", "#0f1720", "#3a4a5c", "#26323f", "#7f93a8", "#5fafaf", "#5c6b7a", "#1f2b38", "#ff875f", "#ffaf87"},
	// For light terminals: dark text, a deep accent.
	{"paper", "Paper", "#af005f", "#ffffff", "#bcbcbc", "#dadada", "#6c6c6c", "#005f87", "#8a8a8a", "#e4e4e4", "#d78700", "#ffaf00"},
}

// Set by applyUI; every view reads them.
var (
	accent, onAccent, edgeColor, ruleColor lipgloss.Color
	randomBg, randomHover                  lipgloss.Color
	faint, heading, button, dragLabel      lipgloss.Style
	selStyle, groupStyle                   lipgloss.Style
)

// darkTerminal is detected once at startup; "auto" picks Ocean on dark, Paper on light.
var darkTerminal = true

func init() { applyUI("") }

// applyUI switches the builder colours. "" means auto.
func applyUI(id string) {
	if id == "" {
		id = "ocean"
		if !darkTerminal {
			id = "paper"
		}
	}
	t := uiThemes[0]
	for _, u := range uiThemes {
		if u.ID == id {
			t = u
		}
	}
	accent, onAccent = lipgloss.Color(t.Accent), lipgloss.Color(t.OnAccent)
	edgeColor, ruleColor = lipgloss.Color(t.Edge), lipgloss.Color(t.Rule)
	randomBg, randomHover = lipgloss.Color(t.Random), lipgloss.Color(t.RandomHover)
	faint = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Faint))
	heading = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(t.Heading))
	button = lipgloss.NewStyle().Padding(0, 1).Background(lipgloss.Color(t.Button))
	dragLabel = lipgloss.NewStyle().Padding(0, 1).Background(accent).Foreground(onAccent)
	selStyle = lipgloss.NewStyle().Background(accent).Foreground(onAccent).Bold(true)
	groupStyle = lipgloss.NewStyle().Width(10).Foreground(lipgloss.Color(t.Group))
}
