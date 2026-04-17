package main

import (
	"fmt"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// Pretty renderer — produces decorated output for ModeTable + TTY
// ─────────────────────────────────────────────────────────────────────────────

type PrettyRenderer struct {
	caps   TTYCaps
	spin   Spinner
	wordmarkPrinted bool
}

func NewPrettyRenderer(caps TTYCaps) *PrettyRenderer {
	return &PrettyRenderer{caps: caps}
}

func (p *PrettyRenderer) FormatEvent(e Event) (string, bool) {
	switch v := e.(type) {
	case *EvtPullStart:
		return p.fmtPullStart(v), false
	case *EvtManifestFetchStart:
		return p.fmtSpinnerLine(FgInfo, "fetching manifest…"), false
	case *EvtManifestFetched:
		return p.fmtManifestFetched(v), false
	case *EvtBlobPreflightStart:
		return p.fmtPreflight(v), false
	case *EvtBlobCached:
		return p.fmtBlobCached(v), false
	case *EvtBlobSkippedLocked:
		return p.fmtBlobSkippedLocked(v), true
	case *EvtBlobResumed:
		return p.fmtBlobResumed(v), false
	case *EvtBlobStart:
		return p.fmtBlobStart(v), false
	case *EvtChunksumsFetched:
		if !v.Available {
			return "", false
		}
		return p.fmtChunksums(v), false
	case *EvtChunkStart:
		return "", false
	case *EvtChunkProgress:
		return p.fmtChunkProgress(v), false
	case *EvtChunkVerified:
		return "", false
	case *EvtChunkMismatch:
		return p.fmtChunkMismatch(v), true
	case *EvtCheckpointSaved:
		return p.fmtCheckpointSaved(v), false
	case *EvtCheckpointCorrupt:
		return p.fmtCheckpointCorrupt(v), true
	case *EvtRetry:
		return p.fmtRetry(v), true
	case *EvtRefresh:
		return p.fmtRefresh(v), true
	case *EvtBlobVerifyStart:
		return p.fmtSpinnerLine(FgInfo, "verifying "+C(p.caps.Color, FgDigest, sha12(v.Blob.Digest))), false
	case *EvtBlobVerified:
		return p.fmtBlobVerified(v), false
	case *EvtBlobVerifyFailed:
		return p.fmtBlobVerifyFailed(v), true
	case *EvtBlobFinalized:
		return p.fmtBlobFinalized(v), false
	case *EvtManifestWriteStart:
		return p.fmtSpinnerLine(FgInfo, "writing manifest…"), false
	case *EvtManifestWritten:
		return p.fmtManifestWritten(v), false
	case *EvtPullCompleted:
		return p.fmtPullCompleted(v), false
	case *EvtPullFailed:
		return p.fmtPullFailed(v), true
	case *EvtInterrupted:
		return p.fmtInterrupted(v), true
	case *EvtListResult:
		return p.fmtList(v), false
	case *EvtShowResult:
		return p.fmtShow(v), false
	case *EvtRmStart:
		return p.fmtRmStart(v), false
	case *EvtRmBlobDeleted:
		return p.fmtRmBlobDeleted(v), false
	case *EvtRmManifestDeleted:
		return p.fmtRmManifestDeleted(v), false
	case *EvtRmCompleted:
		return p.fmtRmCompleted(v), false
	case *EvtCleanStart:
		return p.fmtCleanStart(), false
	case *EvtCleanFileRemoved:
		return p.fmtCleanFileRemoved(v), false
	case *EvtCleanCompleted:
		return p.fmtCleanCompleted(v), false
	case *EvtQueueStart:
		return p.fmtQueueStart(v), false
	case *EvtQueueCompleted:
		return p.fmtQueueCompleted(v), false
	case *EvtQueueFailed:
		return p.fmtQueueFailed(v), true	}
	return "", false
}

func (p *PrettyRenderer) Wordmark() string {
	if p.wordmarkPrinted {
		return ""
	}
	p.wordmarkPrinted = true
	u := p.caps
	return C(u.Color, FgBrand, Wordmark) + "\n"
}

// ── Individual formatters ─────────────────────────────────────────────────────

func (p *PrettyRenderer) fmtPullStart(e *EvtPullStart) string {
	u := p.caps
	hdr := CB(u.Color, FgBrand, "pulling") + " " + C(u.Color, FgFg, e.Model) +
		C(u.Color, FgMuted, ":") + C(u.Color, FgBrandAlt, e.Tag)
	return frameHeader(u, hdr)
}

func (p *PrettyRenderer) fmtSpinnerLine(color, text string) string {
	u := p.caps
	return C(u.Color, FgBrand, boxV(u)) + "  " +
		C(u.Color, color, p.spin.Frame(u)) + " " +
		C(u.Color, color, text)
}

func (p *PrettyRenderer) fmtManifestFetched(e *EvtManifestFetched) string {
	u := p.caps
	return C(u.Color, FgBrand, boxV(u)) + "  " +
		C(u.Color, FgSuccess, glyphCheck(u)) + " " +
		C(u.Color, FgFg, "manifest") + C(u.Color, FgMuted, " · ") +
		C(u.Color, FgFg, fmt.Sprintf("%d blobs", e.BlobCount)) + C(u.Color, FgMuted, " · ") +
		C(u.Color, FgBytes, HumanBytes(e.TotalSize))
}

func (p *PrettyRenderer) fmtPreflight(e *EvtBlobPreflightStart) string {
	u := p.caps
	return C(u.Color, FgBrand, boxV(u)) + "   " +
		C(u.Color, FgMuted, glyphDot(u)) + " " +
		C(u.Color, FgMuted, "preflight "+sha12(e.Blob.Digest))
}

func (p *PrettyRenderer) fmtBlobCached(e *EvtBlobCached) string {
	u := p.caps
	return C(u.Color, FgBrand, boxV(u)) + "  " +
		C(u.Color, FgSuccess, glyphCached(u)) + " " +
		C(u.Color, FgSuccess, "cached") + "    " +
		C(u.Color, FgDigest, sha12(e.Blob.Digest)) + " " +
		C(u.Color, FgBytes, HumanBytes(e.Blob.Size)) + " " +
		C(u.Color, FgMuted, fmt.Sprintf("[%d/%d]", e.Blob.Index, e.Blob.Total))
}

func (p *PrettyRenderer) fmtBlobSkippedLocked(e *EvtBlobSkippedLocked) string {
	u := p.caps
	return C(u.Color, FgWarn, boxV(u)) + "  " +
		C(u.Color, FgWarn, glyphLocked(u)) + " " +
		C(u.Color, FgWarn, "skipped") + "   " +
		C(u.Color, FgDigest, sha12(e.Blob.Digest)) + " " +
		C(u.Color, FgMuted, "lock held by another process")
}

func (p *PrettyRenderer) fmtBlobResumed(e *EvtBlobResumed) string {
	u := p.caps
	return C(u.Color, FgBrand, boxV(u)) + "  " +
		C(u.Color, FgInfo, glyphResumed(u)) + " " +
		C(u.Color, FgInfo, "resumed") + "   " +
		C(u.Color, FgDigest, sha12(e.Blob.Digest)) + " " +
		C(u.Color, FgBytes, "@ "+HumanBytes(e.ResumedAtOffset)) + " " +
		C(u.Color, FgMuted, fmt.Sprintf("(%d/%d chunks)", e.VerifiedChunks, e.TotalChunks))
}

func (p *PrettyRenderer) fmtBlobStart(e *EvtBlobStart) string {
	u := p.caps
	return C(u.Color, FgBrand, boxV(u)) + "  " +
		C(u.Color, FgBrandAlt, glyphArrow(u)) + " " +
		CB(u.Color, FgFg, "downloading") + " " +
		C(u.Color, FgDigest, sha12(e.Blob.Digest)) + " " +
		C(u.Color, FgBytes, HumanBytes(e.Blob.Size)) + " " +
		C(u.Color, FgMuted, fmt.Sprintf("[%d/%d]", e.Blob.Index, e.Blob.Total))
}

func (p *PrettyRenderer) fmtChunksums(e *EvtChunksumsFetched) string {
	u := p.caps
	return C(u.Color, FgBrand, boxV(u)) + "    " +
		C(u.Color, FgMuted, glyphDot(u)) + " " +
		C(u.Color, FgMuted, fmt.Sprintf("chunks: %d", e.ChunkCount))
}

func (p *PrettyRenderer) fmtChunkProgress(e *EvtChunkProgress) string {
	u := p.caps
	pct := 0
	if e.OverallTotal > 0 {
		pct = int(100 * e.OverallDone / e.OverallTotal)
	}
	bar := renderBar(u, pct, e.OverallTotal, e.OverallDone, e.BytesPerSecond, e.ETASeconds)
	return C(u.Color, FgBrand, boxV(u)) + "  " +
		bar + " " +
		CB(u.Color, FgFg, fmt.Sprintf("%d%%", pct)) + " " +
		C(u.Color, FgBytes, HumanBytes(e.OverallDone)+"/"+HumanBytes(e.OverallTotal)) + " " +
		C(u.Color, FgMuted, "·") + " " +
		C(u.Color, FgInfo, HumanBytes(e.BytesPerSecond)+"/s") + " " +
		C(u.Color, FgMuted, "·") + " " +
		C(u.Color, FgETA, "eta "+HumanDuration(time.Duration(e.ETASeconds*1000)*time.Millisecond))
}

func (p *PrettyRenderer) fmtChunkMismatch(e *EvtChunkMismatch) string {
	u := p.caps
	return C(u.Color, FgWarn, boxV(u)) + "  " +
		C(u.Color, FgWarn, glyphCross(u)) + " " +
		C(u.Color, FgWarn, "chunk mismatch") + " " +
		C(u.Color, FgMuted, fmt.Sprintf("at %d-%d, retry %d/%d", e.Chunk.Start, e.Chunk.End, e.Attempt, e.MaxRetry))
}

func (p *PrettyRenderer) fmtCheckpointSaved(e *EvtCheckpointSaved) string {
	u := p.caps
	return C(u.Color, FgBrand, boxV(u)) + "    " +
		C(u.Color, FgMuted, glyphDot(u)) + " " +
		C(u.Color, FgMuted, "checkpoint @ "+HumanBytes(e.VerifiedOffset))
}

func (p *PrettyRenderer) fmtCheckpointCorrupt(e *EvtCheckpointCorrupt) string {
	u := p.caps
	return C(u.Color, FgWarn, boxV(u)) + "  " +
		C(u.Color, FgWarn, glyphLocked(u)) + " " +
		C(u.Color, FgWarn, "checkpoint corrupt") + " " +
		C(u.Color, FgMuted, "— "+e.Reason+", starting fresh")
}

func (p *PrettyRenderer) fmtRetry(e *EvtRetry) string {
	u := p.caps
	return C(u.Color, FgWarn, boxV(u)) + "  " +
		C(u.Color, FgWarn, p.spin.Frame(u)) + " " +
		C(u.Color, FgWarn, fmt.Sprintf("retry %d/%d", e.Attempt, e.MaxAttempts)) + " " +
		C(u.Color, FgMuted, "· "+reasonStr(e.Reason)+" · backoff "+HumanDuration(time.Duration(e.BackoffMs)*time.Millisecond))
}

func (p *PrettyRenderer) fmtRefresh(e *EvtRefresh) string {
	u := p.caps
	var label string
	if e.Kind == RefreshToken {
		label = "refreshing token"
	} else {
		label = "refreshing CDN URL"
	}
	code := "401"
	if e.Kind == RefreshCDN {
		code = "403"
	}
	return C(u.Color, FgInfo, boxV(u)) + "  " +
		C(u.Color, FgInfo, p.spin.Frame(u)) + " " +
		C(u.Color, FgInfo, label) + " " +
		C(u.Color, FgMuted, fmt.Sprintf("(%s, %d/%d)", code, e.Attempt, e.Max))
}

func (p *PrettyRenderer) fmtBlobVerified(e *EvtBlobVerified) string {
	u := p.caps
	return C(u.Color, FgBrand, boxV(u)) + "  " +
		C(u.Color, FgSuccess, glyphCheck(u)) + " " +
		C(u.Color, FgSuccess, "verified") + "  " +
		C(u.Color, FgDigest, sha12(e.Blob.Digest)) + " " +
		C(u.Color, FgMuted, "("+HumanDuration(time.Duration(e.ElapsedMs)*time.Millisecond)+")")
}

func (p *PrettyRenderer) fmtBlobVerifyFailed(e *EvtBlobVerifyFailed) string {
	u := p.caps
	return C(u.Color, FgError, boxV(u)) + "  " +
		C(u.Color, FgError, glyphCross(u)) + " " +
		C(u.Color, FgError, "verify failed") + " " +
		C(u.Color, FgDigest, sha12(e.Blob.Digest)) + " " +
		C(u.Color, FgMuted, fmt.Sprintf("re-download %d/%d", e.Attempt, e.MaxAttempts))
}

func (p *PrettyRenderer) fmtBlobFinalized(e *EvtBlobFinalized) string {
	u := p.caps
	return C(u.Color, FgBrand, boxV(u)) + "  " +
		C(u.Color, FgSuccess, glyphCheck(u)) + " " +
		C(u.Color, FgSuccess, "finalized") + " " +
		C(u.Color, FgDigest, sha12(e.Blob.Digest))
}

func (p *PrettyRenderer) fmtManifestWritten(e *EvtManifestWritten) string {
	u := p.caps
	return C(u.Color, FgBrand, boxV(u)) + "  " +
		C(u.Color, FgSuccess, glyphCheck(u)) + " " +
		C(u.Color, FgFg, "manifest written")
}

func (p *PrettyRenderer) fmtPullCompleted(e *EvtPullCompleted) string {
	return completionBox(p.caps, e.Model, e.Tag, e.TotalBytes, e.ElapsedMs, e.BlobCount)
}

func (p *PrettyRenderer) fmtPullFailed(e *EvtPullFailed) string {
	return errorBox(p.caps, e.Sentinel, e.Message, e.Class)
}

func (p *PrettyRenderer) fmtInterrupted(e *EvtInterrupted) string {
	return interruptBox(p.caps, e.Model, e.Tag)
}

func (p *PrettyRenderer) fmtList(e *EvtListResult) string {
	return listTable(p.caps, e.Rows)
}

func (p *PrettyRenderer) fmtShow(e *EvtShowResult) string {
	return showCard(p.caps, e)
}

func (p *PrettyRenderer) fmtRmStart(e *EvtRmStart) string {
	u := p.caps
	hdr := CB(u.Color, FgBrand, "removing") + " " + C(u.Color, FgFg, e.Model)
	return frameHeader(u, hdr)
}

func (p *PrettyRenderer) fmtRmBlobDeleted(e *EvtRmBlobDeleted) string {
	u := p.caps
	return C(u.Color, FgBrand, boxV(u)) + "  " +
		C(u.Color, FgError, glyphDelete(u)) + " " +
		C(u.Color, FgFg, "blob") + " " +
		C(u.Color, FgDigest, sha12(e.Digest)) + " " +
		C(u.Color, FgMuted, "("+HumanBytes(e.Size)+")")
}

func (p *PrettyRenderer) fmtRmManifestDeleted(e *EvtRmManifestDeleted) string {
	u := p.caps
	return C(u.Color, FgBrand, boxV(u)) + "  " +
		C(u.Color, FgError, glyphDelete(u)) + " " +
		C(u.Color, FgFg, "manifest") + " " +
		C(u.Color, FgFg, e.Model+":"+e.Tag)
}

func (p *PrettyRenderer) fmtRmCompleted(e *EvtRmCompleted) string {
	return rmSummaryBox(p.caps, e.Model, e.BlobsDeleted, e.BytesReclaimed)
}

func (p *PrettyRenderer) fmtCleanStart() string {
	u := p.caps
	hdr := CB(u.Color, FgBrand, "cleaning disposable artifacts")
	return frameHeader(u, hdr)
}

func (p *PrettyRenderer) fmtCleanFileRemoved(e *EvtCleanFileRemoved) string {
	u := p.caps
	return C(u.Color, FgBrand, boxV(u)) + "  " +
		C(u.Color, FgError, glyphDelete(u)) + " " +
		C(u.Color, FgMuted, e.Kind) + " " +
		C(u.Color, FgFg, e.Path) + " " +
		C(u.Color, FgMuted, "("+HumanBytes(e.Size)+")")
}

func (p *PrettyRenderer) fmtCleanCompleted(e *EvtCleanCompleted) string {
	return cleanSummaryBox(p.caps, e.Removed, e.BytesReclaimed)
}

func (p *PrettyRenderer) fmtQueueStart(e *EvtQueueStart) string {
	u := p.caps
	idx := e.Index + 1
	return C(u.Color, FgBrand, "▸ queue") + " " +
		C(u.Color, FgMuted, fmt.Sprintf("[%d/%d]", idx, e.Total)) + " " +
		CB(u.Color, FgBrand, "pulling") + " " +
		C(u.Color, FgFg, e.Model) + C(u.Color, FgMuted, ":") + C(u.Color, FgBrandAlt, e.Tag)
}

func (p *PrettyRenderer) fmtQueueCompleted(e *EvtQueueCompleted) string {
	u := p.caps
	idx := e.Index + 1
	line := C(u.Color, FgBrand, "▸ queue") + " " +
		C(u.Color, FgMuted, fmt.Sprintf("[%d/%d]", idx, e.Total)) + " " +
		C(u.Color, FgSuccess, GlyphCheck) + " " +
		C(u.Color, FgSuccess, "completed") + " " +
		C(u.Color, FgFg, e.Model+":"+e.Tag)
	if e.Failed > 0 {
		line += " " + C(u.Color, FgWarn, fmt.Sprintf("(%d failed)", e.Failed))
	}
	return line
}

func (p *PrettyRenderer) fmtQueueFailed(e *EvtQueueFailed) string {
	u := p.caps
	idx := e.Index + 1
	return C(u.Color, FgBrand, "▸ queue") + " " +
		C(u.Color, FgMuted, fmt.Sprintf("[%d/%d]", idx, e.Total)) + " " +
		C(u.Color, FgError, GlyphCross) + " " +
		C(u.Color, FgError, "failed") + " " +
		C(u.Color, FgFg, e.Model+":"+e.Tag) + " " +
		C(u.Color, FgMuted, "— "+e.Error)
}