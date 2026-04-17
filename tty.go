package main

import (
	"os"
	"strings"

	"golang.org/x/term"
)

// ─────────────────────────────────────────────────────────────────────────────
// TTY capability detection
// ─────────────────────────────────────────────────────────────────────────────

type TTYCaps struct {
	IsTTY       bool
	IsStderrTTY bool
	Color       bool
	Unicode     bool
	Overwrite   bool
	Width       int
	Height      int
}

func DetectTTY() TTYCaps {
	caps := TTYCaps{}
	stdoutFd := os.Stdout.Fd()
	stderrFd := os.Stderr.Fd()

	caps.IsTTY = isTerminal(stdoutFd)
	caps.IsStderrTTY = isTerminal(stderrFd)

	caps.Color = caps.IsTTY
	caps.Unicode = caps.IsTTY
	caps.Overwrite = caps.IsTTY

	if os.Getenv("NO_COLOR") != "" {
		caps.Color = false
	}
	if os.Getenv("TERM") == "dumb" {
		caps.Color = false
		caps.Unicode = false
		caps.Overwrite = false
	}
	if !caps.IsTTY {
		caps.Color = false
		caps.Overwrite = false
		caps.Unicode = false
	}
	if os.Getenv("CI") != "" {
		caps.Overwrite = false
	}
	lang := os.Getenv("LANG")
	if lall := os.Getenv("LC_ALL"); lall != "" {
		lang = lall
	}
	if lang != "" && !strings.Contains(strings.ToUpper(lang), "UTF-8") && !strings.Contains(strings.ToUpper(lang), "UTF8") {
		caps.Unicode = false
	}

	caps.Width, caps.Height = termSize()
	if caps.Width == 0 {
		caps.Width = 80
	}

	return caps
}

func isTerminal(fd uintptr) bool {
	return term.IsTerminal(int(fd))
}

func termSize() (int, int) {
	w, h, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return 0, 0
	}
	return w, h
}

// ─────────────────────────────────────────────────────────────────────────────
// Glyph fallback: returns ASCII substitute when Unicode=false
// ─────────────────────────────────────────────────────────────────────────────

func glyphFor(u TTYCaps, uni, ascii string) string {
	if u.Unicode {
		return uni
	}
	return ascii
}

func boxV(u TTYCaps) string  { return glyphFor(u, BoxV, "|") }
func boxH(u TTYCaps) string  { return glyphFor(u, BoxH, "-") }
func boxTL(u TTYCaps) string { return glyphFor(u, BoxTL, "+") }
func boxTR(u TTYCaps) string { return glyphFor(u, BoxTR, "+") }
func boxBL(u TTYCaps) string { return glyphFor(u, BoxBL, "+") }
func boxBR(u TTYCaps) string { return glyphFor(u, BoxBR, "+") }
func boxML(u TTYCaps) string { return glyphFor(u, BoxML, "+") }
func boxMR(u TTYCaps) string { return glyphFor(u, BoxMR, "+") }

func boxDbH(u TTYCaps) string { return glyphFor(u, BoxDbH, "=") }
func boxDbV(u TTYCaps) string { return glyphFor(u, BoxDbV, "|") }
func boxDbTL(u TTYCaps) string { return glyphFor(u, BoxDbTL, "+") }
func boxDbTR(u TTYCaps) string { return glyphFor(u, BoxDbTR, "+") }
func boxDbBL(u TTYCaps) string { return glyphFor(u, BoxDbBL, "+") }
func boxDbBR(u TTYCaps) string { return glyphFor(u, BoxDbBR, "+") }
func boxDbML(u TTYCaps) string { return glyphFor(u, BoxDbML, "+") }
func boxDbMR(u TTYCaps) string { return glyphFor(u, BoxDbMR, "+") }

func boxHvTL(u TTYCaps) string { return glyphFor(u, BoxHvTL, "#") }
func boxHvTR(u TTYCaps) string { return glyphFor(u, BoxHvTR, "#") }
func boxHvBL(u TTYCaps) string { return glyphFor(u, BoxHvBL, "#") }
func boxHvBR(u TTYCaps) string { return glyphFor(u, BoxHvBR, "#") }
func boxHvH(u TTYCaps) string  { return glyphFor(u, BoxHvH, "#") }
func boxHvV(u TTYCaps) string  { return glyphFor(u, BoxHvV, "|") }

func glyphCheck(u TTYCaps) string   { return glyphFor(u, GlyphCheck, "[ok]") }
func glyphCross(u TTYCaps) string   { return glyphFor(u, GlyphCross, "[x]") }
func glyphArrow(u TTYCaps) string   { return glyphFor(u, GlyphArrow, ">") }
func glyphBullet(u TTYCaps) string  { return glyphFor(u, GlyphBullet, "*") }
func glyphDot(u TTYCaps) string     { return glyphFor(u, GlyphDot, ".") }
func glyphCached(u TTYCaps) string  { return glyphFor(u, GlyphCached, "*") }
func glyphLocked(u TTYCaps) string  { return glyphFor(u, GlyphLocked, "[!]") }
func glyphResumed(u TTYCaps) string { return glyphFor(u, GlyphResumed, "~") }
func glyphDelete(u TTYCaps) string  { return glyphFor(u, GlyphDelete, "-") }
func glyphPlus(u TTYCaps) string   { return glyphFor(u, GlyphPlus, "+") }

func spinnerFrame(u TTYCaps, tick int) string {
	if u.Unicode {
		return SpinnerBraille[tick%len(SpinnerBraille)]
	}
	return "..."
}

func barFill(u TTYCaps) string  { return glyphFor(u, "█", "#") }
func barTrack(u TTYCaps) string { return glyphFor(u, BarTrack, "-") }