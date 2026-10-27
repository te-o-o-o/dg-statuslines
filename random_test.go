package main

import (
	"math/rand/v2"
	"testing"
)

func TestRandomRules(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	for i := range 500 {
		cfg := randomConfig(r)
		if n := len(cfg.Rows); n < 1 || n > 3 {
			t.Fatalf("roll %d: %d rows", i, n)
		}
		if cfg.Theme == "" {
			t.Fatalf("roll %d: no theme", i)
		}
		seen, bars := map[string]bool{}, map[string]bool{}
		for _, row := range cfg.Rows {
			for _, c := range row {
				if c.Widget == "" {
					continue
				}
				if seen[c.Widget] {
					t.Fatalf("roll %d: %s twice", i, c.Widget)
				}
				seen[c.Widget] = true
				if isGauge(c.Widget) {
					bars[c.Bar] = true
				}
			}
		}
		if len(bars) > 1 {
			t.Fatalf("roll %d: gauges mix bar styles %v", i, bars)
		}
	}
}

func TestRollHistory(t *testing.T) {
	m := model{rng: rand.New(rand.NewPCG(3, 4))}
	m.roll()
	first := configJSON(m.cfg)
	m.roll()
	m.cfg.Rows[0][0].Color = "red" // editing the current layout must not touch the history
	if !m.prevRoll() || configJSON(m.cfg) != first {
		t.Fatal("previous roll not restored")
	}
	if m.prevRoll() {
		t.Fatal("no roll before the first one")
	}
	m.roll() // rolling after stepping back drops the later rolls
	if len(m.rolls) != 2 || m.rollIdx != 1 {
		t.Fatalf("history %d at %d", len(m.rolls), m.rollIdx)
	}
}
