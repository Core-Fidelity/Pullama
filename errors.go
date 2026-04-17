package main

import (
	"errors"
	"math"
	"math/rand"
	"net"
	"os"
	"time"
)

var (
	ErrInvalidModelRef   = errors.New("invalid model reference")
	ErrNotFound          = errors.New("not found")
	ErrAuthFailed        = errors.New("authentication failed")
	ErrDiskFull          = errors.New("disk full")
	ErrCheckpointCorrupt = errors.New("checkpoint corrupt")
	ErrChunkMismatch     = errors.New("chunk checksum mismatch")
	ErrBlobMismatch      = errors.New("blob checksum mismatch")
	ErrLockBusy          = errors.New("lock held by another process")
	ErrManifestCorrupt   = errors.New("manifest corrupt")
)

type ErrorClass int

const (
	ClassTransient ErrorClass = iota
	ClassPermanent
	ClassCorrupt
)

func ClassifyError(err error) ErrorClass {
	if errors.Is(err, ErrChunkMismatch) || errors.Is(err, ErrBlobMismatch) || errors.Is(err, ErrCheckpointCorrupt) {
		return ClassCorrupt
	}
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrAuthFailed) || errors.Is(err, ErrDiskFull) || errors.Is(err, os.ErrPermission) {
		return ClassPermanent
	}
	// Transient: connection reset, timeout, unexpected EOF, DNS, HTTP 5xx/429
	if isNetError(err) {
		return ClassTransient
	}
	return ClassTransient
}

func isNetError(err error) bool {
	if _, ok := err.(net.Error); ok {
		return true
	}
	return false
}

const maxRetries = 6

func backoff(attempt int) time.Duration {
	a := attempt
	if a > maxRetries {
		a = maxRetries
	}
	d := time.Duration(int64(math.Pow(2, float64(a)))) * time.Second
	if d > 120*time.Second {
		d = 120 * time.Second
	}
	jitter := 0.5 + rand.Float64()
	return time.Duration(float64(d) * jitter)
}