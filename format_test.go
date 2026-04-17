package main

import (
	"testing"
	"time"
)

func TestHumanBytes(t *testing.T) {
	tests := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{500, "500 B"},
		{1024, "1.0 kB"},
		{4831838208, "4.5 GB"},
		{1048576, "1.0 MB"},
		{1572864, "1.5 MB"},
	}
	for _, tt := range tests {
		got := HumanBytes(tt.in)
		if got != tt.want {
			t.Errorf("HumanBytes(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestHumanDuration(t *testing.T) {
	if got := HumanDuration(754 * time.Second); got != "12m34s" {
		t.Errorf("HumanDuration(754s) = %q, want %q", got, "12m34s")
	}
	if got := HumanDuration(0); got != "0s" {
		t.Errorf("HumanDuration(0) = %q, want %q", got, "0s")
	}
	if got := HumanDuration(90 * time.Minute); got != "1h30m0s" {
		t.Errorf("HumanDuration(90m) = %q, want %q", got, "1h30m0s")
	}
}