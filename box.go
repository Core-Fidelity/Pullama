package main

import (
	"fmt"
	"strings"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// Summary boxes (completion, error, interrupt, rm, clean)
// ─────────────────────────────────────────────────────────────────────────────

// completionBox renders the double-rule pull completed summary
func completionBox(u TTYCaps, model, tag string, totalBytes int64, elapsedMs int64, blobCount int) string {
	var sb strings.Builder
	sb.WriteString(frameClose(u))
	sb.WriteString("\n")

	// Double-rule top
	sb.WriteString(C(u.Color, FgSuccess, boxDbTL(u)+strings.Repeat(boxDbH(u), FrameInnerWidth)+boxDbTR(u)))
	sb.WriteString("\n")

	// Header
	hdr := "  " + glyphCheck(u) + "  pulled " + model + ":" + tag
	sb.WriteString(C(u.Color, FgSuccess, boxDbV(u)))
	sb.WriteString(padStr(hdr, FrameInnerWidth))
	sb.WriteString(C(u.Color, FgSuccess, boxDbV(u)))
	sb.WriteString("\n")

	// Divider
	sb.WriteString(C(u.Color, FgSuccess, boxDbML(u)+strings.Repeat(boxDbH(u), FrameInnerWidth)+boxDbMR(u)))
	sb.WriteString("\n")

	elapsed := HumanDuration(time.Duration(elapsedMs) * time.Millisecond)
	elapsedSec := elapsedMs / 1000
	var avgRate string
	if elapsedSec > 0 {
		avgRate = HumanBytes(totalBytes/elapsedSec) + "/s"
	} else {
		avgRate = HumanBytes(totalBytes) + "/s"
	}

	rows := []struct {
		label string
		value string
		vcol  string
	}{
		{"size", HumanBytes(totalBytes), FgBytes},
		{"blobs", fmt.Sprintf("%d", blobCount), FgFg},
		{"elapsed", elapsed, FgETA},
		{"avg rate", avgRate, FgBytes},
	}

	for _, r := range rows {
		sb.WriteString(C(u.Color, FgSuccess, boxDbV(u)))
		content := "    " + C(u.Color, FgMuted, r.label) + "      " + C(u.Color, r.vcol, r.value)
		sb.WriteString(padStr(content, FrameInnerWidth))
		sb.WriteString(C(u.Color, FgSuccess, boxDbV(u)))
		sb.WriteString("\n")
	}

	sb.WriteString(C(u.Color, FgSuccess, boxDbBL(u)+strings.Repeat(boxDbH(u), FrameInnerWidth)+boxDbBR(u)))
	return sb.String()
}

// errorBox renders the heavy-rule error box
func errorBox(u TTYCaps, sentinel, message string, class ErrorClass) string {
	var sb strings.Builder
	sb.WriteString(frameClose(u))
	sb.WriteString("\n")

	sb.WriteString(C(u.Color, FgError, boxHvTL(u)+strings.Repeat(boxHvH(u), FrameInnerWidth)+boxHvTR(u)))
	sb.WriteString("\n")

	hdr := "  " + glyphCross(u) + "  " + sentinel
	sb.WriteString(C(u.Color, FgError, boxHvV(u)))
	sb.WriteString(padStr(hdr, FrameInnerWidth))
	sb.WriteString(C(u.Color, FgError, boxHvV(u)))
	sb.WriteString("\n")

	sb.WriteString(C(u.Color, FgError, boxHvV(u)+strings.Repeat(boxHvH(u), FrameInnerWidth)+boxHvV(u)))
	sb.WriteString("\n")

	hint := hintByClass(class)
	wrapped := wordWrap(message, FrameInnerWidth-4)
	for _, line := range wrapped {
		sb.WriteString(C(u.Color, FgError, boxHvV(u)))
		sb.WriteString("  ")
		sb.WriteString(C(u.Color, FgFg, line))
		sb.WriteString(padTo("", FrameInnerWidth-4-visualLen(line)))
		sb.WriteString("  ")
		sb.WriteString(C(u.Color, FgError, boxHvV(u)))
		sb.WriteString("\n")
	}

	if hint != "" {
		sb.WriteString(C(u.Color, FgError, boxHvV(u)))
		sb.WriteString(padTo("", FrameInnerWidth))
		sb.WriteString(C(u.Color, FgError, boxHvV(u)))
		sb.WriteString("\n")
		sb.WriteString(C(u.Color, FgError, boxHvV(u)))
		sb.WriteString("  ")
		sb.WriteString(C(u.Color, FgMuted, "hint: "+hint))
		hlen := visualLen("  hint: " + hint)
		sb.WriteString(padTo("", FrameInnerWidth-hlen-2))
		sb.WriteString("  ")
		sb.WriteString(C(u.Color, FgError, boxHvV(u)))
		sb.WriteString("\n")
	}

	sb.WriteString(C(u.Color, FgError, boxHvBL(u)+strings.Repeat(boxHvH(u), FrameInnerWidth)+boxHvBR(u)))
	return sb.String()
}

// interruptBox renders the interrupt box
func interruptBox(u TTYCaps, model, tag string) string {
	var sb strings.Builder
	sb.WriteString(frameClose(u))
	sb.WriteString("\n")

	sb.WriteString(C(u.Color, FgWarn, boxTL(u)+strings.Repeat(boxH(u), FrameInnerWidth)+boxTR(u)))
	sb.WriteString("\n")

	hdr := "  " + glyphLocked(u) + "  interrupted"
	sb.WriteString(C(u.Color, FgWarn, boxV(u)))
	sb.WriteString(padStr(hdr, FrameInnerWidth))
	sb.WriteString(C(u.Color, FgWarn, boxV(u)))
	sb.WriteString("\n")

	sb.WriteString(C(u.Color, FgWarn, boxV(u)+strings.Repeat(boxH(u), FrameInnerWidth)+boxV(u)))
	sb.WriteString("\n")

	resumeCmd := "pullama " + model + ":" + tag
	content := "  resume with:  " + CB(u.Color, FgFg, resumeCmd)
	sb.WriteString(C(u.Color, FgWarn, boxV(u)))
	sb.WriteString(padStr(content, FrameInnerWidth))
	sb.WriteString(C(u.Color, FgWarn, boxV(u)))
	sb.WriteString("\n")

	sb.WriteString(C(u.Color, FgWarn, boxBL(u)+strings.Repeat(boxH(u), FrameInnerWidth)+boxBR(u)))
	return sb.String()
}

// rmSummaryBox renders the rm completion box
func rmSummaryBox(u TTYCaps, model string, blobsDeleted int, bytesReclaimed int64) string {
	var sb strings.Builder
	sb.WriteString(frameClose(u))
	sb.WriteString("\n")

	sb.WriteString(C(u.Color, FgSuccess, boxTL(u)+strings.Repeat(boxH(u), FrameInnerWidth)+boxTR(u)))
	sb.WriteString("\n")

	hdr := "  " + glyphCheck(u) + "  removed " + model
	sb.WriteString(C(u.Color, FgSuccess, boxV(u)))
	sb.WriteString(padStr(hdr, FrameInnerWidth))
	sb.WriteString(C(u.Color, FgSuccess, boxV(u)))
	sb.WriteString("\n")

	sb.WriteString(C(u.Color, FgSuccess, boxV(u)+strings.Repeat(boxH(u), FrameInnerWidth)+boxV(u)))
	sb.WriteString("\n")

	rows := []struct {
		label string
		value string
		vcol  string
	}{
		{"blobs", fmt.Sprintf("%d", blobsDeleted), FgFg},
		{"reclaimed", HumanBytes(bytesReclaimed), FgBytes},
	}
	for _, r := range rows {
		sb.WriteString(C(u.Color, FgSuccess, boxV(u)))
		content := "    " + C(u.Color, FgMuted, r.label) + "      " + C(u.Color, r.vcol, r.value)
		sb.WriteString(padStr(content, FrameInnerWidth))
		sb.WriteString(C(u.Color, FgSuccess, boxV(u)))
		sb.WriteString("\n")
	}

	sb.WriteString(C(u.Color, FgSuccess, boxBL(u)+strings.Repeat(boxH(u), FrameInnerWidth)+boxBR(u)))
	return sb.String()
}

// cleanSummaryBox renders the clean completion box
func cleanSummaryBox(u TTYCaps, removed int, bytesReclaimed int64) string {
	var sb strings.Builder
	sb.WriteString(frameClose(u))
	sb.WriteString("\n")

	sb.WriteString(C(u.Color, FgSuccess, boxTL(u)+strings.Repeat(boxH(u), FrameInnerWidth)+boxTR(u)))
	sb.WriteString("\n")

	hdr := "  " + glyphCheck(u) + "  cleaned"
	sb.WriteString(C(u.Color, FgSuccess, boxV(u)))
	sb.WriteString(padStr(hdr, FrameInnerWidth))
	sb.WriteString(C(u.Color, FgSuccess, boxV(u)))
	sb.WriteString("\n")

	sb.WriteString(C(u.Color, FgSuccess, boxV(u)+strings.Repeat(boxH(u), FrameInnerWidth)+boxV(u)))
	sb.WriteString("\n")

	rows := []struct {
		label string
		value string
		vcol  string
	}{
		{"files", fmt.Sprintf("%d", removed), FgFg},
		{"reclaimed", HumanBytes(bytesReclaimed), FgBytes},
	}
	for _, r := range rows {
		sb.WriteString(C(u.Color, FgSuccess, boxV(u)))
		content := "    " + C(u.Color, FgMuted, r.label) + "      " + C(u.Color, r.vcol, r.value)
		sb.WriteString(padStr(content, FrameInnerWidth))
		sb.WriteString(C(u.Color, FgSuccess, boxV(u)))
		sb.WriteString("\n")
	}

	sb.WriteString(C(u.Color, FgSuccess, boxBL(u)+strings.Repeat(boxH(u), FrameInnerWidth)+boxBR(u)))
	return sb.String()
}

// listTable renders the box-bordered model list
func listTable(u TTYCaps, rows []ModelRow) string {
	const (
		colName = 32
		colSize = 10
		colMod  = 20
		colDig  = 16
	)
	_ = colName + colSize + colMod + colDig // avoid unused warning

var sb strings.Builder
	// Top rule
	sb.WriteString(C(u.Color, FgMuted, boxTL(u)+strings.Repeat(boxH(u), colName)+boxH(u)+strings.Repeat(boxH(u), colSize)+boxH(u)+strings.Repeat(boxH(u), colMod)+boxH(u)+strings.Repeat(boxH(u), colDig)+boxTR(u)))
	sb.WriteString("\n")
	// Header
	sb.WriteString(C(u.Color, FgMuted, boxV(u)))
	sb.WriteString(C(u.Color, FgMuted, fmt.Sprintf(" %-32s", "NAME")))
	sb.WriteString(C(u.Color, FgMuted, boxV(u)))
	sb.WriteString(C(u.Color, FgMuted, fmt.Sprintf("%-10s", "SIZE")))
	sb.WriteString(C(u.Color, FgMuted, boxV(u)))
	sb.WriteString(C(u.Color, FgMuted, fmt.Sprintf(" %-20s", "MODIFIED")))
	sb.WriteString(C(u.Color, FgMuted, boxV(u)))
	sb.WriteString(C(u.Color, FgMuted, fmt.Sprintf(" %-16s", "DIGEST")))
	sb.WriteString(C(u.Color, FgMuted, boxV(u)))
	sb.WriteString("\n")
	// Divider
	sb.WriteString(C(u.Color, FgMuted, boxML(u)+strings.Repeat(boxH(u), colName)+boxH(u)+strings.Repeat(boxH(u), colSize)+boxH(u)+strings.Repeat(boxH(u), colMod)+boxH(u)+strings.Repeat(boxH(u), colDig)+boxMR(u)))
	sb.WriteString("\n")

	if len(rows) == 0 {
		sb.WriteString(C(u.Color, FgMuted, boxV(u)))
		sb.WriteString(C(u.Color, FgMuted, fmt.Sprintf(" %-45s", "(no models)")))
		sb.WriteString(C(u.Color, FgMuted, boxV(u)))
		sb.WriteString("\n")
	} else {
		for _, r := range rows {
			dg := ""
			if len(r.Digest) > 19 {
				dg = r.Digest[7:19]
			}
			sb.WriteString(C(u.Color, FgMuted, boxV(u)))
			sb.WriteString(" " + C(u.Color, FgFg, fmt.Sprintf("%-31s", r.Name)))
			sb.WriteString(C(u.Color, FgMuted, boxV(u)))
			sb.WriteString(" " + C(u.Color, FgBytes, fmt.Sprintf("%9s", HumanBytes(r.Size))))
			sb.WriteString(C(u.Color, FgMuted, boxV(u)))
			sb.WriteString(" " + C(u.Color, FgETA, fmt.Sprintf("%-20s", r.Modified.Format("2006-01-02 15:04:05"))))
			sb.WriteString(C(u.Color, FgMuted, boxV(u)))
			sb.WriteString(" " + C(u.Color, FgDigest, fmt.Sprintf("%-16s", dg)))
			sb.WriteString(C(u.Color, FgMuted, boxV(u)))
			sb.WriteString("\n")
		}
	}

	// Bottom
	sb.WriteString(C(u.Color, FgMuted, boxBL(u)+strings.Repeat(boxH(u), colName)+boxH(u)+strings.Repeat(boxH(u), colSize)+boxH(u)+strings.Repeat(boxH(u), colMod)+boxH(u)+strings.Repeat(boxH(u), colDig)+boxBR(u)))
	return sb.String()
}

// showCard renders the detail card
func showCard(u TTYCaps, e *EvtShowResult) string {
	var sb strings.Builder

	// Header line: ╭─ Name ──────╮
	namePad := FrameInnerWidth - len(e.Name) - 3
	if namePad < 0 {
		namePad = 0
	}
	sb.WriteString(C(u.Color, FgBrand, boxTL(u)+boxH(u)+" "+CB(u.Color, FgBrandAlt, e.Name)+" "+strings.Repeat(boxH(u), namePad)+boxTR(u)))
	sb.WriteString("\n")

	// Blank
	sb.WriteString(frameLineLeft(u, ""))
	sb.WriteString("\n")

	// Fields
	fields := []struct {
		label string
		value string
		vcol  string
	}{
		{"family", e.Family, FgFg},
		{"parameters", e.ParameterSize, FgFg},
		{"quantization", e.Quantization, FgFg},
		{"total size", HumanBytes(e.Size), FgBytes},
	}
	for _, f := range fields {
		sb.WriteString(frameLineLeft(u, "  "+C(u.Color, FgMuted, fmt.Sprintf("%-14s", f.label))+C(u.Color, f.vcol, f.value)))
		sb.WriteString("\n")
	}

	// Blank
	sb.WriteString(frameLineLeft(u, ""))
	sb.WriteString("\n")

	// Layers section
	sb.WriteString(C(u.Color, FgMuted, boxML(u)+boxH(u)+" "+CB(u.Color, FgFg, "layers")+" "+strings.Repeat(boxH(u), FrameInnerWidth-10)+boxMR(u)))
	sb.WriteString("\n")
	sb.WriteString(frameLineLeft(u, ""))
	sb.WriteString("\n")

	for _, l := range e.Layers {
		line := "  " + C(u.Color, FgFg, fmt.Sprintf("%-40s", l.MediaType)) +
			C(u.Color, FgDigest, fmt.Sprintf("%-16s", sha12(l.Digest))) +
			C(u.Color, FgBytes, fmt.Sprintf("%10s", HumanBytes(l.Size)))
		sb.WriteString(frameLineLeft(u, line))
		sb.WriteString("\n")
	}

	sb.WriteString(frameLineLeft(u, ""))
	sb.WriteString("\n")
	sb.WriteString(C(u.Color, FgBrand, boxBL(u)+strings.Repeat(boxH(u), FrameInnerWidth)+boxBR(u)))
	return sb.String()
}

// ── helpers ──────────────────────────────────────────────────────────────────

func hintByClass(class ErrorClass) string {
	switch class {
	case ClassTransient:
		return "retry the command"
	case ClassPermanent:
		return "check your configuration"
	case ClassCorrupt:
		return "partial data was discarded; re-run to redownload"
	}
	return ""
}

func wordWrap(s string, width int) []string {
	if width <= 0 {
		return []string{s}
	}
	var lines []string
	for len(s) > 0 {
		if len(s) <= width {
			lines = append(lines, s)
			break
		}
		// Find last space before width
		idx := strings.LastIndex(s[:width], " ")
		if idx <= 0 {
			idx = width
		}
		lines = append(lines, s[:idx])
		s = strings.TrimLeft(s[idx:], " ")
	}
	return lines
}

func padStr(s string, totalLen int) string {
	vlen := visualLen(s)
	pad := totalLen - vlen
	if pad < 0 {
		pad = 0
	}
	return s + strings.Repeat(" ", pad)
}

func padTo(s string, n int) string {
	if n <= 0 {
		return s
	}
	return s + strings.Repeat(" ", n)
}

