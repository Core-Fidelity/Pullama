package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type ChunkState struct {
	Start          int64  `json:"start"`
	End            int64  `json:"end"`
	ExpectedDigest string `json:"expected_digest,omitempty"`
	Verified       bool   `json:"verified"`
}

type Checkpoint struct {
	Version int          `json:"version"`
	Digest  string       `json:"digest"`
	Size    int64        `json:"size"`
	Chunks  []ChunkState `json:"chunks"`
}

func InitCheckpoint(digest string, size int64, chunks []ChunkState) *Checkpoint {
	return &Checkpoint{
		Version: 1,
		Digest:  digest,
		Size:    size,
		Chunks:  chunks,
	}
}

func VerifiedOffset(chunks []ChunkState) int64 {
	var off int64
	for _, c := range chunks {
		if !c.Verified {
			break
		}
		off = c.End + 1
	}
	return off
}

func checkpointPath(dir, digest string) string {
	// sha256:<hex> → sha256-<hex>
	name := strings.Replace(digest, ":", "-", 1)
	return filepath.Join(dir, ".pullm", name+".json")
}

func SaveCheckpoint(dir string, cp *Checkpoint) error {
	data, err := json.Marshal(cp)
	if err != nil {
		return err
	}
	path := checkpointPath(dir, cp.Digest)
	return atomicWrite(path, data)
}

func LoadCheckpoint(dir, digest string) (*Checkpoint, error) {
	path := checkpointPath(dir, digest)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, ErrCheckpointCorrupt
	}
	var cp Checkpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		return nil, ErrCheckpointCorrupt
	}
	return &cp, nil
}

func DeleteCheckpoint(dir, digest string) error {
	path := checkpointPath(dir, digest)
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// buildChunkPlanFromChunksums creates a fresh ChunkState slice from chunksums,
// with no verified chunks (since the boundary layout changed).
func buildChunkPlanFromChunksums(chunksums []ChunkDigest) []ChunkState {
	chunks := make([]ChunkState, len(chunksums))
	for i, cd := range chunksums {
		chunks[i] = ChunkState{Start: cd.Start, End: cd.End, ExpectedDigest: cd.Digest}
	}
	return chunks
}

// ChunkDigest represents a chunk boundary from the registry chunksums endpoint.
type ChunkDigest struct {
	Digest string `json:"digest"`
	Start  int64  `json:"start"`
	End    int64  `json:"end"`
}

// ValidateCheckpoint performs validations V1–V7 on a loaded checkpoint.
// It mutates cp in place for V7 (partialSize < VerifiedOffset clamping).
// Returns ErrCheckpointCorrupt on any hard failure.
func ValidateCheckpoint(cp *Checkpoint, manifestDigest string, manifestSize int64, chunksums []ChunkDigest, partialSize int64) error {
	// V1: JSON parse — already done by LoadCheckpoint
	// V2: version == 1
	if cp.Version != 1 {
		return ErrCheckpointCorrupt
	}
	// V3: digest matches manifest layer digest
	if cp.Digest != manifestDigest {
		return ErrCheckpointCorrupt
	}
	// V4: size matches manifest layer size
	if cp.Size != manifestSize {
		return ErrCheckpointCorrupt
	}
	// V5: if chunksums available, checkpoint chunk count and boundaries must match.
	// If they don't match (registry returned different chunk layout), the checkpoint
	// is not corrupt per se — we downgrade to a single-chunk plan and mark all
	// previously-verified chunks as unverified, since their digest assignments may
	// no longer be valid. This is a recoverable situation, not data corruption.
	if chunksums != nil {
		if len(cp.Chunks) != len(chunksums) {
			// Chunk count changed — rebuild chunk plan from scratch
			cp.Chunks = buildChunkPlanFromChunksums(chunksums)
			return nil
		}
		boundaryMismatch := false
		for i, ch := range cp.Chunks {
			if ch.Start != chunksums[i].Start || ch.End != chunksums[i].End {
				boundaryMismatch = true
				break
			}
		}
		if boundaryMismatch {
			// Chunk boundaries changed — rebuild
			cp.Chunks = buildChunkPlanFromChunksums(chunksums)
			return nil
		}
	}
	// V6: no verified=true past the contiguous verified prefix (INV-4)
	inPrefix := true
	for _, ch := range cp.Chunks {
		if !inPrefix && ch.Verified {
			return ErrCheckpointCorrupt
		}
		if !ch.Verified {
			inPrefix = false
		}
	}
	// V7: partialSize reconciliation (non-fatal)
	if partialSize >= 0 && partialSize < VerifiedOffset(cp.Chunks) {
		// Clamp: mark chunks unverified until we fit within partialSize
		for i := len(cp.Chunks) - 1; i >= 0; i-- {
			if cp.Chunks[i].Verified && cp.Chunks[i].End+1 > partialSize {
				cp.Chunks[i].Verified = false
			} else {
				break
			}
		}
	}
	return nil
}