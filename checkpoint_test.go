package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVerifiedOffset(t *testing.T) {
	// Empty chunks
	if off := VerifiedOffset(nil); off != 0 {
		t.Errorf("empty: got %d, want 0", off)
	}

	// All verified
	chunks := []ChunkState{
		{Start: 0, End: 99, Verified: true},
		{Start: 100, End: 199, Verified: true},
	}
	if off := VerifiedOffset(chunks); off != 200 {
		t.Errorf("all-verified: got %d, want 200", off)
	}

	// Gap: [v, v, !v, v]
	chunks = []ChunkState{
		{Start: 0, End: 99, Verified: true},
		{Start: 100, End: 199, Verified: true},
		{Start: 200, End: 299, Verified: false},
		{Start: 300, End: 399, Verified: true},
	}
	if off := VerifiedOffset(chunks); off != 200 {
		t.Errorf("gap: got %d, want 200", off)
	}

	// First unverified
	chunks = []ChunkState{
		{Start: 0, End: 99, Verified: false},
	}
	if off := VerifiedOffset(chunks); off != 0 {
		t.Errorf("first-unverified: got %d, want 0", off)
	}
}

func TestSaveLoadCheckpoint(t *testing.T) {
	dir := t.TempDir()
	cp := InitCheckpoint("sha256:abcd", 1000, []ChunkState{
		{Start: 0, End: 499, ExpectedDigest: "sha256:chunk1", Verified: true},
		{Start: 500, End: 999, ExpectedDigest: "", Verified: false},
	})

	if err := SaveCheckpoint(dir, cp); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadCheckpoint(dir, "sha256:abcd")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Version != 1 || loaded.Digest != "sha256:abcd" || loaded.Size != 1000 {
		t.Errorf("round-trip mismatch: %+v", loaded)
	}
	if len(loaded.Chunks) != 2 || !loaded.Chunks[0].Verified || loaded.Chunks[1].Verified {
		t.Errorf("chunks mismatch: %+v", loaded.Chunks)
	}
}

func TestLoadCorruptCheckpoint(t *testing.T) {
	dir := t.TempDir()
	cpPath := filepath.Join(dir, ".pullm", "sha256-abcd.json")
	os.MkdirAll(filepath.Dir(cpPath), 0o755)
	os.WriteFile(cpPath, []byte("{invalid json"), 0o644)

	_, err := LoadCheckpoint(dir, "sha256:abcd")
	if err != ErrCheckpointCorrupt {
		t.Errorf("got %v, want ErrCheckpointCorrupt", err)
	}
}

func TestLoadMissingCheckpoint(t *testing.T) {
	dir := t.TempDir()
	cp, err := LoadCheckpoint(dir, "sha256:missing")
	if err != nil {
		t.Fatal(err)
	}
	if cp != nil {
		t.Error("expected nil for missing checkpoint")
	}
}

func TestCheckpointCrashBeforeRename(t *testing.T) {
	dir := t.TempDir()

	// Save a valid checkpoint first
	cp1 := InitCheckpoint("sha256:abcd", 1000, nil)
	SaveCheckpoint(dir, cp1)

	// Write a .tmp file (simulating crash before rename)
	cpPath := filepath.Join(dir, ".pullm", "sha256-abcd.json")
	os.WriteFile(cpPath+".tmp", []byte("junk"), 0o644)

	// The existing valid .json should still load
	loaded, err := LoadCheckpoint(dir, "sha256:abcd")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Digest != "sha256:abcd" {
		t.Error("original checkpoint should still be loadable after crash")
	}
}

func TestValidateCheckpointVersion(t *testing.T) {
	cp := &Checkpoint{Version: 0, Digest: "sha256:abc", Size: 100}
	if err := ValidateCheckpoint(cp, "sha256:abc", 100, nil, -1); err != ErrCheckpointCorrupt {
		t.Errorf("version 0: got %v, want ErrCheckpointCorrupt", err)
	}
	cp.Version = 99
	if err := ValidateCheckpoint(cp, "sha256:abc", 100, nil, -1); err != ErrCheckpointCorrupt {
		t.Errorf("version 99: got %v, want ErrCheckpointCorrupt", err)
	}
}

func TestValidateCheckpointDigestMismatch(t *testing.T) {
	cp := &Checkpoint{Version: 1, Digest: "sha256:abc", Size: 100}
	if err := ValidateCheckpoint(cp, "sha256:different", 100, nil, -1); err != ErrCheckpointCorrupt {
		t.Errorf("digest mismatch: got %v, want ErrCheckpointCorrupt", err)
	}
}

func TestValidateCheckpointSizeMismatch(t *testing.T) {
	cp := &Checkpoint{Version: 1, Digest: "sha256:abc", Size: 100}
	if err := ValidateCheckpoint(cp, "sha256:abc", 200, nil, -1); err != ErrCheckpointCorrupt {
		t.Errorf("size mismatch: got %v, want ErrCheckpointCorrupt", err)
	}
}

func TestValidateCheckpointChunksumBoundaryChange(t *testing.T) {
	cp := &Checkpoint{
		Version: 1, Digest: "sha256:abc", Size: 200,
		Chunks: []ChunkState{
			{Start: 0, End: 99, Verified: true},
			{Start: 100, End: 199, Verified: false},
		},
	}
	// Different boundaries — should rebuild chunk plan, not return corrupt
	chunksums := []ChunkDigest{
		{Digest: "sha256:x", Start: 0, End: 49},
		{Digest: "sha256:y", Start: 50, End: 199},
	}
	if err := ValidateCheckpoint(cp, "sha256:abc", 200, chunksums, -1); err != nil {
		t.Errorf("boundary change: got %v, want nil (rebuild)", err)
	}
	if len(cp.Chunks) != 2 {
		t.Errorf("expected 2 rebuilt chunks, got %d", len(cp.Chunks))
	}
	for _, ch := range cp.Chunks {
		if ch.Verified {
			t.Errorf("rebuilt chunk should not be verified: %+v", ch)
		}
	}
}

func TestValidateCheckpointVerifiedPastPrefix(t *testing.T) {
	cp := &Checkpoint{
		Version: 1, Digest: "sha256:abc", Size: 300,
		Chunks: []ChunkState{
			{Start: 0, End: 99, Verified: true},
			{Start: 100, End: 199, Verified: false},
			{Start: 200, End: 299, Verified: true}, // past prefix!
		},
	}
	if err := ValidateCheckpoint(cp, "sha256:abc", 300, nil, -1); err != ErrCheckpointCorrupt {
		t.Errorf("verified-past-prefix: got %v, want ErrCheckpointCorrupt", err)
	}
}

func TestValidateCheckpointPartialSizeClamp(t *testing.T) {
	cp := &Checkpoint{
		Version: 1, Digest: "sha256:abc", Size: 300,
		Chunks: []ChunkState{
			{Start: 0, End: 99, Verified: true},
			{Start: 100, End: 199, Verified: true},
			{Start: 200, End: 299, Verified: true},
		},
	}
	// partialSize < VerifiedOffset (300) → should clamp
	err := ValidateCheckpoint(cp, "sha256:abc", 300, nil, 150)
	if err != nil {
		t.Errorf("partialSize clamp: got %v, want nil", err)
	}
	// Chunks should be mutated: chunk[1] still verified (end 199 >= 150... wait, End+1=200 > 150)
	// Actually: chunk[2].End+1 = 300 > 150 → unverified
	// chunk[1].End+1 = 200 > 150 → unverified
	// chunk[0].End+1 = 100 ≤ 150 → stays verified
	if !cp.Chunks[0].Verified {
		t.Error("chunk 0 should remain verified")
	}
	if cp.Chunks[1].Verified {
		t.Error("chunk 1 should be unverified after clamp")
	}
	if cp.Chunks[2].Verified {
		t.Error("chunk 2 should be unverified after clamp")
	}
}