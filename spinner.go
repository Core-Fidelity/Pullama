package main

import "sync/atomic"

// ─────────────────────────────────────────────────────────────────────────────
// Spinner state — tick is advanced by the render loop
// ─────────────────────────────────────────────────────────────────────────────

type Spinner struct {
	tick atomic.Int32
}

func (s *Spinner) Frame(u TTYCaps) string {
	return spinnerFrame(u, int(s.tick.Load()))
}

func (s *Spinner) Advance() {
	s.tick.Add(1)
}