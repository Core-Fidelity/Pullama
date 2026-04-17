package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// ── Event IR (closed set) ─────────────────────────────────────────────────────

type Event interface{ eventMarker() }

type ExitCode int

const (
	ExitOK       ExitCode = 0
	ExitGeneral  ExitCode = 1
	ExitDiskFull ExitCode = 2
	ExitAuthFail ExitCode = 3
	ExitNotFound ExitCode = 4
)

type Phase int

const (
	PhasePull Phase = iota
	PhaseVerify
	PhaseFinalize
	PhaseClean
)

type BlobRef struct {
	Digest    string
	Size      int64
	MediaType string
	Index     int
	Total     int
}

type ChunkRef struct {
	BlobDigest string
	Start      int64
	End        int64
	Index      int
	Total      int
}

type RetryReason int

const (
	RetryConnReset RetryReason = iota
	RetryTimeout
	RetryShortBody
	RetryHTTP5xx
	RetryHTTP429
	RetryDNS
	RetryChunkMismatch
	RetryBlobMismatch
)

type RefreshKind int

const (
	RefreshToken RefreshKind = iota
	RefreshCDN
)

type EvtPullStart struct{ Model, Tag string }
type EvtManifestFetchStart struct{ Model, Tag string }
type EvtManifestFetched struct {
	Model     string
	Tag       string
	BlobCount int
	TotalSize int64
}

type EvtBlobPreflightStart struct{ Blob BlobRef }
type EvtBlobCached struct{ Blob BlobRef }
type EvtBlobSkippedLocked struct{ Blob BlobRef }
type EvtBlobResumed struct {
	Blob            BlobRef
	ResumedAtOffset int64
	VerifiedChunks  int
	TotalChunks     int
}

type EvtBlobStart struct{ Blob BlobRef }
type EvtChunksumsFetched struct {
	Blob       BlobRef
	ChunkCount int
	Available  bool
}

type EvtChunkStart struct{ Chunk ChunkRef }
type EvtChunkProgress struct {
	Chunk          ChunkRef
	BytesReceived  int64
	BytesTotal     int64
	BlobBytesDone  int64
	BlobBytesTotal int64
	OverallDone    int64
	OverallTotal   int64
	BytesPerSecond int64
	ETASeconds     int64
}

type EvtChunkVerified struct {
	Chunk     ChunkRef
	HadDigest bool
	ElapsedMs int64
}

type EvtChunkMismatch struct {
	Chunk    ChunkRef
	Attempt  int
	MaxRetry int
}

type EvtCheckpointSaved struct {
	BlobDigest     string
	VerifiedOffset int64
}

type EvtCheckpointCorrupt struct {
	BlobDigest string
	Reason     string
}

type EvtRetry struct {
	Blob         BlobRef
	Chunk        ChunkRef
	Attempt      int
	MaxAttempts  int
	Reason       RetryReason
	BackoffMs    int64
	HTTPStatus   int
	RetryAfterMs int64
}

type EvtRefresh struct {
	Blob    BlobRef
	Kind    RefreshKind
	Attempt int
	Max     int
}

type EvtBlobVerifyStart struct{ Blob BlobRef }
type EvtBlobVerified struct {
	Blob      BlobRef
	ElapsedMs int64
}

type EvtBlobVerifyFailed struct {
	Blob        BlobRef
	Attempt     int
	MaxAttempts int
}

type EvtBlobFinalized struct{ Blob BlobRef }
type EvtManifestWriteStart struct{ Model, Tag string }
type EvtManifestWritten struct{ Model, Tag string }
type EvtPullCompleted struct {
	Model      string
	Tag        string
	TotalBytes int64
	ElapsedMs  int64
	BlobCount  int
}

type EvtPullFailed struct {
	Model    string
	Tag      string
	Class    ErrorClass
	Sentinel string
	Message  string
}

type EvtInterrupted struct{ Model, Tag string }

// List/Show/Rm/Clean events
type ModelRow struct {
	Name     string
	Size     int64
	Modified time.Time
	Digest   string
}

type EvtListResult struct{ Rows []ModelRow }

type LayerRow struct {
	MediaType string
	Digest    string
	Size      int64
}

type EvtShowResult struct {
	Name          string
	Family        string
	ParameterSize string
	Quantization  string
	Size          int64
	Config        LayerRow
	Layers        []LayerRow
}

type EvtRmStart struct{ Model string }
type EvtRmBlobDeleted struct {
	Digest string
	Size   int64
}

type EvtRmManifestDeleted struct{ Model, Tag string }
type EvtRmCompleted struct {
	Model          string
	BlobsDeleted   int
	BytesReclaimed int64
}

type EvtCleanStart struct{}
type EvtCleanFileRemoved struct {
	Path string
	Kind string
	Size int64
}

type EvtCleanCompleted struct {
	Removed        int
	BytesReclaimed int64
}

// Queue events
type EvtQueueStart struct {
	Index   int // which entry in the queue (0-based)
	Total   int // total entries in the queue
	Model   string
	Tag     string
}

type EvtQueueCompleted struct {
	Index    int
	Total    int
	Model    string
	Tag      string
	Failed   int // count of failed entries so far
}

type EvtQueueFailed struct {
	Index    int
	Total    int
	Model    string
	Tag      string
	Error    string
	Failed   int // count of failed entries so far
}

// Marker implementations
func (EvtPullStart) eventMarker()          {}
func (EvtManifestFetchStart) eventMarker() {}
func (EvtManifestFetched) eventMarker()    {}
func (EvtBlobPreflightStart) eventMarker() {}
func (EvtBlobCached) eventMarker()         {}
func (EvtBlobSkippedLocked) eventMarker()  {}
func (EvtBlobResumed) eventMarker()        {}
func (EvtBlobStart) eventMarker()          {}
func (EvtChunksumsFetched) eventMarker()   {}
func (EvtChunkStart) eventMarker()         {}
func (EvtChunkProgress) eventMarker()      {}
func (EvtChunkVerified) eventMarker()      {}
func (EvtChunkMismatch) eventMarker()      {}
func (EvtCheckpointSaved) eventMarker()    {}
func (EvtCheckpointCorrupt) eventMarker()  {}
func (EvtRetry) eventMarker()              {}
func (EvtRefresh) eventMarker()            {}
func (EvtBlobVerifyStart) eventMarker()    {}
func (EvtBlobVerified) eventMarker()       {}
func (EvtBlobVerifyFailed) eventMarker()   {}
func (EvtBlobFinalized) eventMarker()      {}
func (EvtManifestWriteStart) eventMarker() {}
func (EvtManifestWritten) eventMarker()    {}
func (EvtPullCompleted) eventMarker()      {}
func (EvtPullFailed) eventMarker()         {}
func (EvtInterrupted) eventMarker()        {}
func (EvtListResult) eventMarker()         {}
func (EvtShowResult) eventMarker()         {}
func (EvtRmStart) eventMarker()            {}
func (EvtRmBlobDeleted) eventMarker()      {}
func (EvtRmManifestDeleted) eventMarker()  {}
func (EvtRmCompleted) eventMarker()        {}
func (EvtCleanStart) eventMarker()         {}
func (EvtCleanFileRemoved) eventMarker()   {}
func (EvtCleanCompleted) eventMarker()      {}
func (EvtQueueStart) eventMarker()        {}
func (EvtQueueCompleted) eventMarker()    {}
func (EvtQueueFailed) eventMarker()      {}
// ── OutputMode & State ────────────────────────────────────────────────────────

type OutputMode int

const (
	ModeTable OutputMode = iota
	ModeCompact
	ModeJSON
	ModeDebug
)

type State int

const (
	StateIdle State = iota
	StateInitializing
	StateDownloading
	StateVerifying
	StateFinalizing
	StateCompleted
	StateFailed
)

type RenderAction int

const (
	ActionNone RenderAction = iota
	ActionAppendLine
	ActionOverwriteLine
	ActionClearLine
	ActionStderrLine
	ActionEmitJSON
	ActionEmitDebug
	ActionEmitTable
)

// ── UI ────────────────────────────────────────────────────────────────────────

type UI struct {
	cfg       *Config
	mode      OutputMode
	state     State
	stdout    io.Writer
	stderr    io.Writer
	isatty    bool
	lastLine  string // for overwrite
	progStart time.Time
	pretty    *PrettyRenderer
	caps      TTYCaps
	wordmark  bool
}

func NewUI(cfg *Config) *UI {
	caps := DetectTTY()
	if cfg.NoColor {
		caps.Color = false
	}
	return &UI{
		cfg:      cfg,
		mode:     ModeTable,
		stdout:   os.Stdout,
		stderr:   os.Stderr,
		isatty:   caps.IsTTY,
		caps:     caps,
		pretty:   NewPrettyRenderer(caps),
		wordmark: true,
	}
}

func (u *UI) Emit(e Event) {
	if u == nil {
		return
	}
	action, next := u.transition(e)
	u.state = next
	u.render(e, action)
}

// Teardown restores terminal state on exit
func (u *UI) Teardown() {
	if u.caps.Color && u.caps.IsTTY {
		fmt.Fprint(u.stdout, CursorShow+Reset+"\n")
	}
}

func (u *UI) transition(e Event) (RenderAction, State) {
	switch u.state {
	case StateIdle:
		switch e.(type) {
		case *EvtPullStart:
			return ActionAppendLine, StateInitializing
		case *EvtListResult, *EvtShowResult:
			return ActionAppendLine, StateIdle
		case *EvtRmStart, *EvtRmBlobDeleted, *EvtRmManifestDeleted, *EvtRmCompleted:
			return ActionAppendLine, StateIdle
		case *EvtCleanStart, *EvtCleanFileRemoved, *EvtCleanCompleted:
			return ActionAppendLine, StateIdle
		case *EvtQueueStart, *EvtQueueCompleted, *EvtQueueFailed:
			return ActionAppendLine, StateIdle
		}
	case StateInitializing:
		switch e.(type) {
		case *EvtManifestFetchStart, *EvtManifestFetched, *EvtBlobPreflightStart, *EvtBlobCached:
			return ActionAppendLine, StateInitializing
		case *EvtBlobSkippedLocked:
			return ActionStderrLine, StateInitializing
		case *EvtBlobResumed:
			return ActionAppendLine, StateDownloading
		case *EvtBlobStart:
			return ActionAppendLine, StateDownloading
		case *EvtPullFailed:
			return ActionStderrLine, StateFailed
		case *EvtInterrupted:
			return ActionStderrLine, StateFailed
		}
	case StateDownloading:
		switch e.(type) {
		case *EvtChunksumsFetched, *EvtBlobCached, *EvtBlobResumed, *EvtBlobStart, *EvtBlobPreflightStart:
			return ActionAppendLine, StateDownloading
		case *EvtChunkStart:
			return ActionNone, StateDownloading
		case *EvtChunkProgress:
			return ActionOverwriteLine, StateDownloading
		case *EvtChunkVerified:
			return ActionNone, StateDownloading
		case *EvtChunkMismatch, *EvtCheckpointCorrupt, *EvtRetry, *EvtRefresh, *EvtBlobSkippedLocked:
			return ActionStderrLine, StateDownloading
		case *EvtCheckpointSaved:
			return ActionNone, StateDownloading
		case *EvtBlobVerifyStart:
			return ActionAppendLine, StateVerifying
		case *EvtManifestWriteStart:
			return ActionAppendLine, StateFinalizing
		case *EvtPullFailed:
			return ActionStderrLine, StateFailed
		case *EvtInterrupted:
			return ActionStderrLine, StateFailed
		}
	case StateVerifying:
		switch e.(type) {
		case *EvtBlobVerified:
			return ActionAppendLine, StateFinalizing
		case *EvtBlobVerifyFailed:
			return ActionStderrLine, StateDownloading
		case *EvtPullFailed:
			return ActionStderrLine, StateFailed
		case *EvtInterrupted:
			return ActionStderrLine, StateFailed
		}
	case StateFinalizing:
		switch e.(type) {
		case *EvtBlobFinalized:
			return ActionAppendLine, StateDownloading
		case *EvtBlobPreflightStart:
			return ActionAppendLine, StateInitializing
		case *EvtManifestWriteStart, *EvtManifestWritten:
			return ActionAppendLine, StateFinalizing
		case *EvtPullCompleted:
			return ActionAppendLine, StateCompleted
		case *EvtPullFailed:
			return ActionStderrLine, StateFailed
		case *EvtInterrupted:
			return ActionStderrLine, StateFailed
		}
	}
	return ActionNone, u.state
}

func (u *UI) render(e Event, action RenderAction) {
	if action == ActionNone {
		return
	}
	if u.mode == ModeJSON {
		data, _ := json.Marshal(e)
		fmt.Fprintln(u.stdout, string(data))
		return
	}
	if u.mode == ModeDebug {
		fmt.Fprintf(u.stdout, "%+v\n", e)
		return
	}

	var line string
	var toStderr bool

	if u.mode == ModeTable && u.caps.Color {
		// Wordmark before first pull
		if u.wordmark {
			switch e.(type) {
			case *EvtPullStart:
				wm := u.pretty.Wordmark()
				if wm != "" {
					fmt.Fprint(u.stdout, wm)
				}
				u.wordmark = false
			}
		}
		// Cursor hide for spinner-bearing events
		if u.caps.Overwrite && isSpinnerEvent(e) {
			fmt.Fprint(u.stdout, CursorHide)
		}
		// Advance spinner on every emit
		u.pretty.spin.Advance()
		line, toStderr = u.pretty.FormatEvent(e)
	} else {
		line, toStderr = u.formatEvent(e)
	}

	if toStderr {
		action = ActionStderrLine
	}

	// Verbose gating
	name := eventName(e)
	if u.cfg.Verbose {
		// show everything
	} else if verboseOnly(name) {
		return
	}

	// Quiet gating
	if u.cfg.Quiet && quietSuppress(name) {
		return
	}

	// Overwrite → AppendLine fallback for non-tty
	if action == ActionOverwriteLine && (!u.isatty || u.mode != ModeTable) {
		if u.mode == ModeCompact {
			action = ActionOverwriteLine // compact always overwrites
		} else {
			action = ActionAppendLine
		}
	}

	w := u.stdout
	if action == ActionStderrLine || toStderr {
		w = u.stderr
	}

	// Multi-line output (boxes): always append
	if strings.Contains(line, "\n") {
		if u.lastLine != "" && u.isatty {
			fmt.Fprint(u.stdout, "\n")
			u.lastLine = ""
		}
		fmt.Fprint(w, line+"\n")
		return
	}

	switch action {
	case ActionOverwriteLine:
		if u.isatty && u.caps.Overwrite {
			// Write new content first, then clear the tail to avoid flicker.
			// \r moves to column 0, we print the new line, then \x1b[0K
			// clears only the remainder — no gap between clear and content.
			fmt.Fprintf(w, "\r%s\x1b[0K", line)
		} else {
			fmt.Fprintln(w, line)
		}
		u.lastLine = line
	case ActionAppendLine, ActionStderrLine:
		if u.lastLine != "" && u.isatty {
			fmt.Fprint(u.stdout, "\n")
			u.lastLine = ""
		}
		fmt.Fprintln(w, line)
	case ActionEmitTable:
		fmt.Fprintln(w, line)
	}
}

func (u *UI) formatEvent(e Event) (string, bool) {
	switch v := e.(type) {
	case *EvtPullStart:
		return fmt.Sprintf("pulling %s:%s", v.Model, v.Tag), false
	case *EvtManifestFetchStart:
		return "  fetching manifest", false
	case *EvtManifestFetched:
		return fmt.Sprintf("  manifest: %d blobs, %s", v.BlobCount, HumanBytes(v.TotalSize)), false
	case *EvtBlobPreflightStart:
		return fmt.Sprintf("  blob %d/%d %s %s", v.Blob.Index, v.Blob.Total, sha12(v.Blob.Digest), HumanBytes(v.Blob.Size)), false
	case *EvtBlobCached:
		return fmt.Sprintf("  blob %d/%d %s cached", v.Blob.Index, v.Blob.Total, sha12(v.Blob.Digest)), false
	case *EvtBlobSkippedLocked:
		return fmt.Sprintf("warning: skipping %s — lock held by another process", v.Blob.Digest), true
	case *EvtBlobResumed:
		return fmt.Sprintf("  blob %d/%d %s resumed at %s (%d/%d)", v.Blob.Index, v.Blob.Total, sha12(v.Blob.Digest), HumanBytes(v.ResumedAtOffset), v.VerifiedChunks, v.TotalChunks), false
	case *EvtBlobStart:
		return fmt.Sprintf("  blob %d/%d %s downloading", v.Blob.Index, v.Blob.Total, sha12(v.Blob.Digest)), false
	case *EvtChunksumsFetched:
		if v.Available {
			return fmt.Sprintf("  chunks: %d", v.ChunkCount), false
		}
		return "", false
	case *EvtChunkStart:
		return "", false
	case *EvtChunkProgress:
		pct := 0
		if v.OverallTotal > 0 {
			pct = int(100 * v.OverallDone / v.OverallTotal)
		}
		bar := progressBar(pct)
		if u.mode == ModeCompact {
			return fmt.Sprintf("%d%% %s/%s", pct, HumanBytes(v.OverallDone), HumanBytes(v.OverallTotal)), false
		}
		return fmt.Sprintf("[%s] %d%%  %s/%s  %s",
			bar, pct, HumanBytes(v.OverallDone), HumanBytes(v.OverallTotal),
			HumanDuration(time.Duration(v.ETASeconds*1000)*time.Millisecond)), false
	case *EvtChunkVerified:
		return "", false
	case *EvtChunkMismatch:
		return fmt.Sprintf("chunk hash mismatch at %d-%d, retry %d/%d", v.Chunk.Start, v.Chunk.End, v.Attempt, v.MaxRetry), true
	case *EvtCheckpointSaved:
		return fmt.Sprintf("checkpoint saved %s @ %s", sha12(v.BlobDigest), HumanBytes(v.VerifiedOffset)), false
	case *EvtCheckpointCorrupt:
		return fmt.Sprintf("checkpoint corrupt for %s: %s — starting fresh", sha12(v.BlobDigest), v.Reason), true
	case *EvtRetry:
		return fmt.Sprintf("retry %d/%d: %s, backing off %s", v.Attempt, v.MaxAttempts, reasonStr(v.Reason), HumanDuration(time.Duration(v.BackoffMs)*time.Millisecond)), true
	case *EvtRefresh:
		if v.Kind == RefreshToken {
			return fmt.Sprintf("regenerating auth token (401 from registry) [%d/%d]", v.Attempt, v.Max), true
		}
		return fmt.Sprintf("refreshing CDN URL (403) [%d/%d]", v.Attempt, v.Max), true
	case *EvtBlobVerifyStart:
		return fmt.Sprintf("  verifying %s", sha12(v.Blob.Digest)), false
	case *EvtBlobVerified:
		return fmt.Sprintf("  verified  %s (%s)", sha12(v.Blob.Digest), HumanDuration(time.Duration(v.ElapsedMs)*time.Millisecond)), false
	case *EvtBlobVerifyFailed:
		return fmt.Sprintf("full-blob verify failed %s, re-download %d/%d", sha12(v.Blob.Digest), v.Attempt, v.MaxAttempts), true
	case *EvtBlobFinalized:
		return fmt.Sprintf("  finalized %s", sha12(v.Blob.Digest)), false
	case *EvtManifestWriteStart:
		return "  writing manifest", false
	case *EvtManifestWritten:
		return "  manifest written", false
	case *EvtPullCompleted:
		return fmt.Sprintf("completed %s:%s  %s in %s", v.Model, v.Tag, HumanBytes(v.TotalBytes), HumanDuration(time.Duration(v.ElapsedMs)*time.Millisecond)), false
	case *EvtPullFailed:
		return fmt.Sprintf("error: %s", v.Message), true
	case *EvtInterrupted:
		return fmt.Sprintf("interrupted — resume with: pullama %s:%s", v.Model, v.Tag), true
	case *EvtListResult:
		return u.formatList(v), false
	case *EvtShowResult:
		return u.formatShow(v), false
	case *EvtRmStart:
		return fmt.Sprintf("removing %s", v.Model), false
	case *EvtRmBlobDeleted:
		return fmt.Sprintf("  deleted blob %s (%s)", sha12(v.Digest), HumanBytes(v.Size)), false
	case *EvtRmManifestDeleted:
		return fmt.Sprintf("  deleted manifest %s:%s", v.Model, v.Tag), false
	case *EvtRmCompleted:
		return fmt.Sprintf("removed %s — %d blobs, %s reclaimed", v.Model, v.BlobsDeleted, HumanBytes(v.BytesReclaimed)), false
	case *EvtCleanStart:
		return "cleaning disposable artifacts", false
	case *EvtCleanFileRemoved:
		return fmt.Sprintf("  removed %s %s (%s)", v.Kind, v.Path, HumanBytes(v.Size)), false
	case *EvtCleanCompleted:
		return fmt.Sprintf("clean: %d files, %s reclaimed", v.Removed, HumanBytes(v.BytesReclaimed)), false
	case *EvtQueueStart:
		return fmt.Sprintf("queue [%d/%d] pulling %s:%s", v.Index+1, v.Total, v.Model, v.Tag), false
	case *EvtQueueCompleted:
		return fmt.Sprintf("queue [%d/%d] completed %s:%s (%d failed)", v.Index+1, v.Total, v.Model, v.Tag, v.Failed), false
	case *EvtQueueFailed:
		return fmt.Sprintf("queue [%d/%d] failed %s:%s: %s", v.Index+1, v.Total, v.Model, v.Tag, v.Error), true
	}
	return "", false
}

func (u *UI) formatList(e *EvtListResult) string {
	var b strings.Builder
	for _, r := range e.Rows {
		dg := ""
		if len(r.Digest) > 16 {
			dg = r.Digest[7:19] // sha256: prefix + 12 hex
		}
		fmt.Fprintf(&b, "%-32s  %-10s  %-20s  %-16s\n", r.Name, HumanBytes(r.Size), r.Modified.Format("2006-01-02 15:04:05"), dg)
	}
	return b.String()
}

func (u *UI) formatShow(e *EvtShowResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Name:           %s\n", e.Name)
	fmt.Fprintf(&b, "Family:         %s\n", e.Family)
	fmt.Fprintf(&b, "Parameters:     %s\n", e.ParameterSize)
	fmt.Fprintf(&b, "Quantization:   %s\n", e.Quantization)
	fmt.Fprintf(&b, "Size:           %s\n", HumanBytes(e.Size))
	return b.String()
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func sha12(digest string) string {
	if len(digest) > 19 {
		return digest[7:19] // skip "sha256:", take 12 hex
	}
	return digest
}

func progressBar(pct int) string {
	width := 24
	filled := width * pct / 100
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

func reasonStr(r RetryReason) string {
	switch r {
	case RetryConnReset:
		return "connection reset"
	case RetryTimeout:
		return "timeout"
	case RetryShortBody:
		return "short body"
	case RetryHTTP5xx:
		return "server error"
	case RetryHTTP429:
		return "rate limited"
	case RetryDNS:
		return "dns failure"
	case RetryChunkMismatch:
		return "chunk checksum mismatch"
	case RetryBlobMismatch:
		return "blob checksum mismatch"
	}
	return "unknown"
}

func eventName(e Event) string {
	return strings.TrimPrefix(fmt.Sprintf("%T", e), "*")
}

func verboseOnly(name string) bool {
	return name == "EvtCheckpointSaved" || name == "EvtChunksumsFetched" ||
		name == "EvtBlobPreflightStart" || name == "EvtManifestFetchStart" ||
		name == "EvtManifestWriteStart"
}

func quietSuppress(name string) bool {
	switch name {
	case "EvtPullStart", "EvtPullCompleted", "EvtPullFailed", "EvtInterrupted":
		return false
	default:
		return true
	}
}

func isSpinnerEvent(e Event) bool {
	switch e.(type) {
	case *EvtManifestFetchStart, *EvtBlobVerifyStart, *EvtManifestWriteStart, *EvtRetry, *EvtRefresh:
		return true
	}
	return false
}