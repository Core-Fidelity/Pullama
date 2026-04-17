package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyBlob(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.blob")

	content := []byte("hello world")
	h := sha256.New()
	h.Write(content)
	digest := "sha256:" + hex.EncodeToString(h.Sum(nil))

	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	ok, err := VerifyBlob(path, digest)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected match")
	}

	ok, err = VerifyBlob(path, "sha256:baddigest")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected mismatch")
	}
}

func TestVerifyBlobEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.blob")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	emptyDigest := "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	ok, err := VerifyBlob(path, emptyDigest)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("empty file should match SHA256 of empty string")
	}
}

func TestVerifyChunk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chunk.blob")

	// 128 bytes of known data
	data := make([]byte, 128)
	for i := range data {
		data[i] = byte(i)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	// Verify first 64 bytes
	h := sha256.New()
	h.Write(data[:64])
	digest := "sha256:" + hex.EncodeToString(h.Sum(nil))

	ok, err := VerifyChunk(path, 0, 64, digest)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("chunk should match")
	}

	ok, err = VerifyChunk(path, 0, 64, "sha256:wrong")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("chunk should not match wrong digest")
	}
}