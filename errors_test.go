package main

import (
	"io"
	"net"
	"os"
	"testing"
	"time"
)

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

var _ net.Error = timeoutErr{}

func TestClassifyError(t *testing.T) {
	tests := []struct {
		err   error
		class ErrorClass
	}{
		{ErrChunkMismatch, ClassCorrupt},
		{ErrBlobMismatch, ClassCorrupt},
		{ErrCheckpointCorrupt, ClassCorrupt},
		{ErrNotFound, ClassPermanent},
		{ErrAuthFailed, ClassPermanent},
		{ErrDiskFull, ClassPermanent},
		{os.ErrPermission, ClassPermanent},
		{timeoutErr{}, ClassTransient},
		{io.ErrUnexpectedEOF, ClassTransient},
	}
	for _, tt := range tests {
		got := ClassifyError(tt.err)
		if got != tt.class {
			t.Errorf("ClassifyError(%v) = %v, want %v", tt.err, got, tt.class)
		}
	}
}

func TestBackoff(t *testing.T) {
	for i := 0; i <= 20; i++ {
		d := backoff(i)
		if d < 0 || d > 180*time.Second {
			t.Errorf("backoff(%d) = %v, out of range", i, d)
		}
	}
	// backoff(0) should be roughly 1s ± jitter
	d := backoff(0)
	if d < 500*time.Millisecond || d > 1500*time.Millisecond {
		t.Errorf("backoff(0) = %v, expected ~1s", d)
	}
}