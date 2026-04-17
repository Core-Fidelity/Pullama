package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const blobStateSkip = "skip"
const blobStateDownload = "download"

type blobPreflight struct {
	state      string
	checkpoint *Checkpoint
	lock       *Lock
	partial    *os.File
	cached     bool
	locked     bool
	resumed    bool
	resumedAt  int64
	verifiedN  int
	totalN     int
}

func preflight(cfg *Config, layer ManifestLayer, ref ModelRef, ui *UI, blobIdx, blobTotal int) (blobPreflight, error) {
	blobsDir := filepath.Join(cfg.ModelsDir, "blobs")
	os.MkdirAll(blobsDir, 0o755)
	digestName := strings.Replace(layer.Digest, ":", "-", 1)
	finalPath := filepath.Join(blobsDir, digestName)
	blob := BlobRef{Digest: layer.Digest, Size: layer.Size, MediaType: layer.MediaType, Index: blobIdx, Total: blobTotal}

	ui.Emit(&EvtBlobPreflightStart{Blob: blob})

	if _, err := os.Stat(finalPath); err == nil {
		ok, err := VerifyBlob(finalPath, layer.Digest)
		if err != nil {
			return blobPreflight{}, err
		}
		if ok {
			ui.Emit(&EvtBlobCached{Blob: blob})
			return blobPreflight{state: blobStateSkip, cached: true}, nil
		}
		os.Remove(finalPath)
	}

	lockPath := filepath.Join(blobsDir, digestName+".lock")
	lock, err := AcquireLock(lockPath)
	if err != nil {
		if err == ErrLockBusy {
			ui.Emit(&EvtBlobSkippedLocked{Blob: blob})
			return blobPreflight{state: blobStateSkip, locked: true}, nil
		}
		return blobPreflight{}, err
	}

	cp, err := LoadCheckpoint(cfg.ModelsDir, layer.Digest)
	if err == ErrCheckpointCorrupt {
		ui.Emit(&EvtCheckpointCorrupt{BlobDigest: layer.Digest, Reason: "JSON parse failure"})
		DeleteCheckpoint(cfg.ModelsDir, layer.Digest)
		cp = nil
		err = nil
	}
	if err != nil {
		lock.Release()
		return blobPreflight{}, err
	}

	partialPath := filepath.Join(blobsDir, digestName+".partial")
	f, err := os.OpenFile(partialPath, os.O_RDWR, 0o644)
	if err != nil {
		if !os.IsNotExist(err) {
			lock.Release()
			return blobPreflight{}, err
		}
		f, err = os.OpenFile(partialPath, os.O_CREATE|os.O_RDWR, 0o644)
		if err != nil {
			lock.Release()
			return blobPreflight{}, err
		}
		if err := f.Truncate(layer.Size); err != nil {
			f.Close()
			lock.Release()
			return blobPreflight{}, err
		}
	} else {
		if info, _ := f.Stat(); info.Size() != layer.Size {
			if err := f.Truncate(layer.Size); err != nil {
				f.Close()
				lock.Release()
				return blobPreflight{}, err
			}
		}
	}

	pf := blobPreflight{state: blobStateDownload, checkpoint: cp, lock: lock, partial: f}
	if cp != nil {
		pf.resumed = true
		pf.resumedAt = VerifiedOffset(cp.Chunks)
		vn := 0
		for _, ch := range cp.Chunks {
			if ch.Verified {
				vn++
			}
		}
		pf.verifiedN = vn
		pf.totalN = len(cp.Chunks)
	}
	return pf, nil
}

var errCDNStale = fmt.Errorf("CDN URL stale (403)")
var errTokenStale = fmt.Errorf("auth token stale (401)")

// speedTracker computes download throughput using a sliding window of buckets,
// matching ollama's proven approach: up to 10 buckets, sampled at most once per second.
type speedBucket struct {
	updated time.Time
	value   int64
}

type speedTracker struct {
	initialValue int64
	started      time.Time
	buckets      []speedBucket
	maxBuckets   int
}

func newSpeedTracker(overallDone int64) *speedTracker {
	return &speedTracker{
		initialValue: overallDone,
		started:      time.Now(),
		maxBuckets:   10,
	}
}

// set records the current overall downloaded bytes. Buckets are throttled to 1 per second.
func (s *speedTracker) set(overallDone int64) {
	if len(s.buckets) == 0 || time.Since(s.buckets[len(s.buckets)-1].updated) > time.Second {
		s.buckets = append(s.buckets, speedBucket{
			updated: time.Now(),
			value:   overallDone,
		})
		if len(s.buckets) > s.maxBuckets {
			s.buckets = s.buckets[1:]
		}
	}
}

// rate returns the current throughput in bytes/sec using the sliding window.
func (s *speedTracker) rate() int64 {
	var numerator, denominator float64

	switch len(s.buckets) {
	case 0:
		// no data yet
	case 1:
		numerator = float64(s.buckets[0].value - s.initialValue)
		denominator = s.buckets[0].updated.Sub(s.started).Round(time.Second).Seconds()
	default:
		first, last := s.buckets[0], s.buckets[len(s.buckets)-1]
		numerator = float64(last.value - first.value)
		denominator = last.updated.Sub(first.updated).Round(time.Second).Seconds()
	}

	if denominator > 0 {
		return int64(numerator / denominator)
	}
	return 0
}

func downloadChunk(ctx context.Context, cfg *Config, url string, f *os.File, ch ChunkState, ui *UI, chunk ChunkRef, blob BlobRef, overallDone, overallTotal int64, speed *speedTracker) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", ch.Start, ch.End))

	resp, err := (&http.Client{Timeout: cfg.Timeout}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 403:
		return errCDNStale
	case 401:
		return errTokenStale
	case 200, 206:
	default:
		if resp.StatusCode >= 500 {
			return fmt.Errorf("server error: HTTP %d", resp.StatusCode)
		}
		return fmt.Errorf("unexpected HTTP %d", resp.StatusCode)
	}

	h := sha256.New()
	w := io.NewOffsetWriter(f, ch.Start)
	mw := io.MultiWriter(h, w)
	chunkSize := ch.End - ch.Start + 1

	var copied int64
	buf := make([]byte, 32*1024)
	lastEmit := time.Now()

	for copied < chunkSize {
		remaining := chunkSize - copied
		toRead := int64(len(buf))
		if remaining < toRead {
			toRead = remaining
		}
		n, err := resp.Body.Read(buf[:toRead])
		if n > 0 {
			written, wErr := mw.Write(buf[:n])
			copied += int64(written)
			if wErr != nil {
				return wErr
			}
		}
		if err != nil {
			if err == io.EOF {
				if copied < chunkSize {
					return fmt.Errorf("short body: got %d bytes, want %d", copied, chunkSize)
				}
				break
			}
			return err
		}

		now := time.Now()
		if now.Sub(lastEmit) >= 100*time.Millisecond {
			lastEmit = now
			curDone := overallDone + ch.Start + copied
			speed.set(curDone)
			bps := speed.rate()
			var eta int64
			left := overallTotal - curDone
			if bps > 0 {
				eta = left / bps
			}
			ui.Emit(&EvtChunkProgress{
				Chunk:          chunk,
				BytesReceived:  copied,
				BytesTotal:     chunkSize,
				BlobBytesDone:  ch.Start + copied,
				BlobBytesTotal: ch.End + 1,
				OverallDone:    curDone,
				OverallTotal:   overallTotal,
				BytesPerSecond: bps,
				ETASeconds:     eta,
			})
		}
	}

	// Emit final progress event so the bar shows final position for this chunk
	curDone := overallDone + ch.Start + copied
	speed.set(curDone)
	bps := speed.rate()
	var eta int64
	left := overallTotal - curDone
	if bps > 0 {
		eta = left / bps
	}
	ui.Emit(&EvtChunkProgress{
		Chunk:          chunk,
		BytesReceived:  copied,
		BytesTotal:     chunkSize,
		BlobBytesDone:  ch.Start + copied,
		BlobBytesTotal: ch.End + 1,
		OverallDone:    overallDone + ch.Start + copied,
		OverallTotal:   overallTotal,
		BytesPerSecond: bps,
		ETASeconds:     eta,
	})

	if ch.ExpectedDigest != "" {
		gotDigest := "sha256:" + hex.EncodeToString(h.Sum(nil))
		if !strings.EqualFold(gotDigest, ch.ExpectedDigest) {
			return ErrChunkMismatch
		}
	}
	return f.Sync()
}

func resume(cfg *Config, cp *Checkpoint, f *os.File, chunksums []ChunkDigest) error {
	blobsDir := filepath.Join(cfg.ModelsDir, "blobs")
	for {
		off := VerifiedOffset(cp.Chunks)
		if off == 0 {
			if err := f.Truncate(0); err != nil {
				return err
			}
			if err := f.Sync(); err != nil {
				return err
			}
			fsyncDir(blobsDir)
			return nil
		}
		if err := f.Truncate(off); err != nil {
			return err
		}
		if err := f.Sync(); err != nil {
			return err
		}
		fsyncDir(blobsDir)

		idx := 0
		for i, ch := range cp.Chunks {
			if ch.Verified {
				idx = i
			} else {
				break
			}
		}
		last := cp.Chunks[idx]
		if last.ExpectedDigest != "" {
			partialPath := filepath.Join(blobsDir, strings.Replace(cp.Digest, ":", "-", 1)+".partial")
			ok, err := VerifyChunk(partialPath, last.Start, last.End-last.Start+1, last.ExpectedDigest)
			if err != nil {
				return err
			}
			if !ok {
				cp.Chunks[idx].Verified = false
				continue
			}
		}
		return nil
	}
}

func buildChunkPlan(layer ManifestLayer, chunksums []ChunkDigest) []ChunkState {
	if len(chunksums) > 0 {
		chunks := make([]ChunkState, len(chunksums))
		for i, cd := range chunksums {
			chunks[i] = ChunkState{Start: cd.Start, End: cd.End, ExpectedDigest: cd.Digest}
		}
		return chunks
	}
	return []ChunkState{{Start: 0, End: layer.Size - 1}}
}

func countVerified(chunks []ChunkState) int {
	n := 0
	for _, ch := range chunks {
		if ch.Verified {
			n++
		} else {
			break
		}
	}
	return n
}

func DownloadBlob(ctx context.Context, cfg *Config, layer ManifestLayer, ref ModelRef, ui *UI, blobIdx, blobTotal int, overallDone, overallTotal int64) error {
	const maxFullRedownloads = 2
	blobsDir := filepath.Join(cfg.ModelsDir, "blobs")
	digestName := strings.Replace(layer.Digest, ":", "-", 1)
	blob := BlobRef{Digest: layer.Digest, Size: layer.Size, MediaType: layer.MediaType, Index: blobIdx, Total: blobTotal}

	for redownload := 0; redownload <= maxFullRedownloads; redownload++ {
		pf, err := preflight(cfg, layer, ref, ui, blobIdx, blobTotal)
		if err != nil {
			return err
		}
		if pf.state == blobStateSkip {
			return nil
		}
		defer pf.partial.Close()

		// Don't emit resume/start yet — checkpoint validation may discard
		// the checkpoint and change the actual behavior. Emit after validation.

		chunksums, err := FetchChunksums(ctx, cfg, ref, layer.Digest)
		if err != nil {
			return err
		}
		ui.Emit(&EvtChunksumsFetched{Blob: blob, ChunkCount: len(chunksums), Available: len(chunksums) > 0})

		var cp *Checkpoint
		resumed := pf.resumed
		if pf.checkpoint != nil {
			if info, _ := pf.partial.Stat(); info != nil {
				verr := ValidateCheckpoint(pf.checkpoint, layer.Digest, layer.Size, chunksums, info.Size())
				if verr == ErrCheckpointCorrupt {
					reason := fmt.Sprintf("validation failed (cp chunks=%d, chunksums=%d, partialSize=%d, verifiedOff=%d)",
						len(pf.checkpoint.Chunks), len(chunksums), info.Size(), VerifiedOffset(pf.checkpoint.Chunks))
					ui.Emit(&EvtCheckpointCorrupt{BlobDigest: layer.Digest, Reason: reason})
					DeleteCheckpoint(cfg.ModelsDir, layer.Digest)
					pf.checkpoint = nil
					resumed = false
				} else if verr != nil {
					return verr
				}
			}
		}

		if pf.checkpoint != nil {
			cp = pf.checkpoint
			if err := resume(cfg, cp, pf.partial, chunksums); err != nil {
				return err
			}
		} else {
			cp = InitCheckpoint(layer.Digest, layer.Size, buildChunkPlan(layer, chunksums))
			if err := SaveCheckpoint(cfg.ModelsDir, cp); err != nil {
				return err
			}
		}

		// Emit the correct event after checkpoint validation decides the actual path
		if resumed {
			ui.Emit(&EvtBlobResumed{Blob: blob, ResumedAtOffset: VerifiedOffset(cp.Chunks), VerifiedChunks: countVerified(cp.Chunks), TotalChunks: len(cp.Chunks)})
		} else {
			ui.Emit(&EvtBlobStart{Blob: blob})
		}

		cdnURL, err := ResolveBlobURL(ctx, cfg, ref, layer.Digest)
		if err != nil {
			pf.lock.Release()
			return err
		}

		tokenRefreshes, cdnRefreshes := 0, 0
		speed := newSpeedTracker(overallDone)
		chunkTotal := len(cp.Chunks)

		for i, ch := range cp.Chunks {
			if ch.Verified {
				continue
			}
			chunkRef := ChunkRef{BlobDigest: layer.Digest, Start: ch.Start, End: ch.End, Index: i, Total: chunkTotal}
			ui.Emit(&EvtChunkStart{Chunk: chunkRef})

			for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}

				dlErr := downloadChunk(ctx, cfg, cdnURL, pf.partial, cp.Chunks[i], ui, chunkRef, blob, overallDone, overallTotal, speed)
				if dlErr == nil {
					cp.Chunks[i].Verified = true
					ui.Emit(&EvtChunkVerified{Chunk: chunkRef, HadDigest: ch.ExpectedDigest != ""})
					if saveErr := SaveCheckpoint(cfg.ModelsDir, cp); saveErr != nil {
						return saveErr
					}
					ui.Emit(&EvtCheckpointSaved{BlobDigest: layer.Digest, VerifiedOffset: VerifiedOffset(cp.Chunks)})
					break
				}

				if dlErr == errCDNStale || strings.Contains(dlErr.Error(), "403") {
					cdnRefreshes++
					ui.Emit(&EvtRefresh{Blob: blob, Kind: RefreshCDN, Attempt: cdnRefreshes, Max: 5})
					if cdnRefreshes > 5 {
						return fmt.Errorf("CDN URL refresh limit exceeded")
					}
					cdnURL, err = ResolveBlobURL(ctx, cfg, ref, layer.Digest)
					if err != nil {
						return err
					}
					attempt--
					continue
				}

				if dlErr == errTokenStale || strings.Contains(dlErr.Error(), "401") {
					tokenRefreshes++
					ui.Emit(&EvtRefresh{Blob: blob, Kind: RefreshToken, Attempt: tokenRefreshes, Max: 3})
					if tokenRefreshes > 3 {
						return ErrAuthFailed
					}
					attempt--
					continue
				}

				if dlErr == ErrChunkMismatch {
					ui.Emit(&EvtChunkMismatch{Chunk: chunkRef, Attempt: attempt, MaxRetry: cfg.MaxRetries})
					prevEnd := int64(-1)
					if i > 0 && cp.Chunks[i-1].Verified {
						prevEnd = cp.Chunks[i-1].End
					}
					tOff := prevEnd + 1
					if tOff < 0 {
						tOff = 0
					}
					if err := pf.partial.Truncate(tOff); err != nil {
						return err
					}
					if err := pf.partial.Sync(); err != nil {
						return err
					}
					fsyncDir(blobsDir)
					continue
				}

				cls := ClassifyError(dlErr)
				if cls == ClassPermanent || cls == ClassCorrupt {
					return dlErr
				}

				bd := backoff(attempt)
				ui.Emit(&EvtRetry{Blob: blob, Chunk: chunkRef, Attempt: attempt, MaxAttempts: cfg.MaxRetries, Reason: classifyRetry(dlErr), BackoffMs: bd.Milliseconds()})
				if attempt == cfg.MaxRetries {
					return fmt.Errorf("chunk %d-%d: max retries (%d) exceeded: %w", ch.Start, ch.End, cfg.MaxRetries, dlErr)
				}
				select {
				case <-time.After(bd):
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}

		partialPath := filepath.Join(blobsDir, digestName+".partial")
		ui.Emit(&EvtBlobVerifyStart{Blob: blob})
		ok, err := VerifyBlob(partialPath, layer.Digest)
		if err != nil {
			return err
		}
		if !ok {
			ui.Emit(&EvtBlobVerifyFailed{Blob: blob, Attempt: redownload + 1, MaxAttempts: maxFullRedownloads})
			pf.partial.Close()
			os.Remove(partialPath)
			DeleteCheckpoint(cfg.ModelsDir, layer.Digest)
			pf.lock.Release()
			if redownload == maxFullRedownloads {
				return ErrBlobMismatch
			}
			continue
		}
		ui.Emit(&EvtBlobVerified{Blob: blob})

		finalPath := filepath.Join(blobsDir, digestName)
		if err := os.Rename(partialPath, finalPath); err != nil {
			return err
		}
		fsyncDir(blobsDir)
		pf.lock.Release()
		DeleteCheckpoint(cfg.ModelsDir, layer.Digest)
		ui.Emit(&EvtBlobFinalized{Blob: blob})
		return nil
	}
	return ErrBlobMismatch
}

func classifyRetry(err error) RetryReason {
	cls := ClassifyError(err)
	if cls == ClassCorrupt {
		if err == ErrChunkMismatch {
			return RetryChunkMismatch
		}
		return RetryBlobMismatch
	}
	msg := err.Error()
	if strings.Contains(msg, "connection reset") {
		return RetryConnReset
	}
	if strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline") {
		return RetryTimeout
	}
	if strings.Contains(msg, "short body") {
		return RetryShortBody
	}
	if strings.Contains(msg, "server error") {
		return RetryHTTP5xx
	}
	if strings.Contains(msg, "rate limited") || strings.Contains(msg, "429") {
		return RetryHTTP429
	}
	if strings.Contains(msg, "dns") || strings.Contains(msg, "no such host") {
		return RetryDNS
	}
	return RetryConnReset
}