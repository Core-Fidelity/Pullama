package main

import "strings"

// ─────────────────────────────────────────────────────────────────────────────
// Frame lifecycle — open/close boxes with continuation prefix
// ─────────────────────────────────────────────────────────────────────────────

const (
	FrameInnerWidth = 45
	FrameLeftPad    = 2
	FrameRightPad   = 2
)

// frameOpen renders ╭─ pulling model:tag ─╮
func frameOpen(u TTYCaps, label, detail string) string {
	left := boxTL(u) + boxH(u)
	right := boxH(u) + boxTR(u)
	content := label + " " + detail
	// Pad content to fit FrameInnerWidth
	contentLen := visualLen(content)
	targetInner := FrameInnerWidth
	if contentLen < targetInner {
		content += strings.Repeat(" ", targetInner-contentLen)
	}
	if contentLen > targetInner {
		content = content[:targetInner]
	}
	return C(u.Color, FgBrand, left) + content + C(u.Color, FgBrand, right)
}

// frameHeader renders ╭─ B{label} ───╮ with proper padding
func frameHeader(u TTYCaps, content string) string {
	clen := visualLen(content)
	padLen := FrameInnerWidth - clen - 2 // 2 for spaces around content
	if padLen < 1 {
		padLen = 1
	}
	return C(u.Color, FgBrand, boxTL(u)+boxH(u)) + " " + content + " " + strings.Repeat(boxH(u), padLen) + C(u.Color, FgBrand, boxTR(u))
}

// frameClose renders ╰──────────────────────────╯
func frameClose(u TTYCaps) string {
	inner := strings.Repeat(boxH(u), FrameInnerWidth)
	return C(u.Color, FgBrand, boxBL(u)+inner+boxBR(u))
}

// frameLine renders a content line inside a frame: │  content  │
func frameLine(u TTYCaps, content string) string {
	prefix := C(u.Color, FgBrand, boxV(u)) + "  "
	suffix := "  " + C(u.Color, FgBrand, boxV(u))
	clen := visualLen(content)
	pad := FrameInnerWidth - clen
	if pad < 0 {
		pad = 0
	}
	return prefix + content + strings.Repeat(" ", pad) + suffix
}

// frameLineLeft renders │ content│ (no right pad, for tables)
func frameLineLeft(u TTYCaps, content string) string {
	prefix := C(u.Color, FgBrand, boxV(u)) + " "
	suffix := " " + C(u.Color, FgBrand, boxV(u))
	clen := visualLen(content)
	pad := FrameInnerWidth - clen
	if pad < 0 {
		pad = 0
	}
	return prefix + content + strings.Repeat(" ", pad) + suffix
}

// frameDivider renders ├──────────────────────────┤
func frameDivider(u TTYCaps) string {
	inner := strings.Repeat(boxH(u), FrameInnerWidth)
	return C(u.Color, FgMuted, boxML(u)+inner+boxMR(u))
}

// visualLen counts printable runes, stripping ANSI escapes
func visualLen(s string) int {
	n := 0
	inESC := false
	for _, r := range s {
		if r == '\x1b' {
			inESC = true
			continue
		}
		if inESC {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inESC = false
			}
			continue
		}
		n++
	}
	return n
}

// statusLine renders │  glyph label detail
func statusLine(u TTYCaps, glyphColor, glyph, labelColor, label, detail string) string {
	return C(u.Color, FgBrand, boxV(u)) + "  " +
		C(u.Color, glyphColor, glyph) + " " +
		C(u.Color, labelColor, label) + " " +
		detail
}