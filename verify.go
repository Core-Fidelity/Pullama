package main

import (
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"io"
	"os"
	"strings"
)

func VerifyBlob(path, expectedDigest string) (bool, error) {
	h := sha256.New()
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	if _, err := io.Copy(h, f); err != nil {
		return false, err
	}
	return digestMatch(h, expectedDigest), nil
}

func VerifyChunk(path string, offset, length int64, expectedDigest string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	r := io.NewSectionReader(f, offset, length)
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return false, err
	}
	return digestMatch(h, expectedDigest), nil
}

func digestMatch(h hash.Hash, expected string) bool {
	got := "sha256:" + hex.EncodeToString(h.Sum(nil))
	return strings.EqualFold(got, expected)
}