package main

// ─────────────────────────────────────────────────────────────────────────────
// 1.1 SGR escape sequences (raw bytes)
// ─────────────────────────────────────────────────────────────────────────────

const (
	CSI   = "\x1b["
	Reset = "\x1b[0m"

	Bold      = "\x1b[1m"
	Dim       = "\x1b[2m"
	Italic    = "\x1b[3m"
	Underline = "\x1b[4m"

	CR            = "\r"
	LF            = "\n"
	ClearLine     = "\x1b[2K"
	ClearToEOL    = "\x1b[0K"
	CursorUp1     = "\x1b[1A"
	CursorHide    = "\x1b[?25l"
	CursorShow    = "\x1b[?25h"
	SaveCursor    = "\x1b7"
	RestoreCursor = "\x1b8"
)

// ─────────────────────────────────────────────────────────────────────────────
// 1.2 Palette (24-bit truecolor)
// ─────────────────────────────────────────────────────────────────────────────

const (
	FgBrand    = "\x1b[38;2;139;92;246m"
	FgBrandAlt = "\x1b[38;2;192;132;252m"

	FgSuccess = "\x1b[38;2;34;197;94m"
	FgWarn    = "\x1b[38;2;234;179;8m"
	FgError   = "\x1b[38;2;239;68;68m"
	FgInfo    = "\x1b[38;2;56;189;248m"
	FgMuted   = "\x1b[38;2;148;163;184m"
	FgSubtle  = "\x1b[38;2;100;116;139m"
	FgFg      = "\x1b[38;2;226;232;240m"
	FgDigest  = "\x1b[38;2;251;191;36m"
	FgBytes   = "\x1b[38;2;167;243;208m"
	FgETA     = "\x1b[38;2;196;181;253m"

	FgBarStart = "\x1b[38;2;139;92;246m"
	FgBarEnd   = "\x1b[38;2;56;189;248m"
	FgBarTrack = "\x1b[38;2;51;65;85m"
)

// ─────────────────────────────────────────────────────────────────────────────
// 1.3 Box-drawing characters
// ─────────────────────────────────────────────────────────────────────────────

const (
	BoxTL = "╭"
	BoxTR = "╮"
	BoxBL = "╰"
	BoxBR = "╯"
	BoxH  = "─"
	BoxV  = "│"
	BoxML = "├"
	BoxMR = "┤"

	BoxHvTL = "┏"
	BoxHvTR = "┓"
	BoxHvBL = "┗"
	BoxHvBR = "┛"
	BoxHvH  = "━"
	BoxHvV  = "┃"

	BoxDbTL = "╔"
	BoxDbTR = "╗"
	BoxDbBL = "╚"
	BoxDbBR = "╝"
	BoxDbH  = "═"
	BoxDbV  = "║"
	BoxDbML = "╠"
	BoxDbMR = "╣"
)

// ─────────────────────────────────────────────────────────────────────────────
// 1.4 Progress bar glyphs
// ─────────────────────────────────────────────────────────────────────────────

var BarEighths = [9]string{
	" ", "▏", "▎", "▍", "▌", "▋", "▊", "▉", "█",
}

const BarTrack = "░"

// ─────────────────────────────────────────────────────────────────────────────
// 1.5 Spinner frames (Braille, 10fps)
// ─────────────────────────────────────────────────────────────────────────────

var SpinnerBraille = [10]string{
	"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏",
}

const SpinnerIntervalMs = 80

const (
	GlyphCheck   = "✓"
	GlyphCross   = "✗"
	GlyphArrow   = "›"
	GlyphBullet  = "•"
	GlyphDot     = "·"
	GlyphCached  = "◆"
	GlyphLocked  = "⚠"
	GlyphResumed = "↻"
	GlyphDelete  = "✖"
	GlyphPlus    = "＋"
)

// ─────────────────────────────────────────────────────────────────────────────
// 1.6 Wordmark
// ─────────────────────────────────────────────────────────────────────────────

const Wordmark = "  ▸ core-fidelity - pullama"

const WordmarkCompact = "  ▸ core-fidelity - pullama"

// ─────────────────────────────────────────────────────────────────────────────
// Color helper: wrap s in color escape + reset
// ─────────────────────────────────────────────────────────────────────────────

// C conditionally wraps s in color. colorOk controls whether ANSI is emitted;
// when false, returns s unmodified (no-color / non-TTY fallback).
func C(colorOk bool, color, s string) string {
	if !colorOk || color == "" {
		return s
	}
	return color + s + Reset
}

// CB wraps in color + bold.
func CB(colorOk bool, color, s string) string {
	if !colorOk {
		return s
	}
	if color == "" {
		return Bold + s + Reset
	}
	return color + Bold + s + Reset
}