package main

import "strings"

// ─────────────────────────────────────────────────────────────────────────────
// Gradient progress bar with sub-cell precision
// ─────────────────────────────────────────────────────────────────────────────

const (
	BarWidthDefault = 30
	BarWidthNarrow  = 20
	BarWidthTiny    = 12
)

func barWidth(u TTYCaps) int {
	if u.Width < 60 {
		return BarWidthTiny
	}
	if u.Width < 100 {
		return BarWidthNarrow
	}
	return BarWidthDefault
}

func renderBar(u TTYCaps, pct int, totalBytes, doneBytes int64, bps int64, etaSec int64) string {
	w := barWidth(u)
	progress := float64(pct) / 100.0
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}

	fullCells := int(progress * float64(w))
	if fullCells > w {
		fullCells = w
	}
	partial := int((progress*float64(w) - float64(fullCells)) * 8)
	if partial < 0 {
		partial = 0
	}
	if partial > 8 {
		partial = 8
	}
	emptyCells := w - fullCells
	if partial > 0 && fullCells < w {
		emptyCells--
	}
	if emptyCells < 0 {
		emptyCells = 0
	}

	var sb strings.Builder
	sb.WriteString(C(u.Color, FgMuted, "["))

	if pct >= 100 {
		// All filled in success color
		sb.WriteString(C(u.Color, FgSuccess, strings.Repeat(barFill(u), fullCells)))
	} else if u.Color && u.Unicode {
		// Gradient fill
		sb.WriteString(C(true, FgBarStart, strings.Repeat("█", fullCells)))
		if partial > 0 && fullCells < w {
			sb.WriteString(C(true, FgBarEnd, BarEighths[partial]))
		}
		sb.WriteString(C(true, FgBarTrack, strings.Repeat(BarTrack, emptyCells)))
	} else {
		sb.WriteString(strings.Repeat(barFill(u), fullCells))
		if partial > 0 && fullCells < w {
			sb.WriteString(barFill(u))
		}
		sb.WriteString(strings.Repeat(barTrack(u), emptyCells))
	}

	sb.WriteString(C(u.Color, FgMuted, "]"))
	return sb.String()
}