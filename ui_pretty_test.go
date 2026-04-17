package main

import (
	"bytes"
	"testing"
)

func TestPrettyRendererPullSequence(t *testing.T) {
	caps := TTYCaps{IsTTY: true, Color: true, Unicode: true, Overwrite: true, Width: 120, Height: 40}
	cfg := DefaultConfig()
	cfg.NoColor = false
	ui := &UI{
		cfg:     cfg,
		mode:    ModeTable,
		stdout:  &bytes.Buffer{},
		stderr:  &bytes.Buffer{},
		isatty:  true,
		caps:    caps,
		pretty:  NewPrettyRenderer(caps),
		wordmark: true,
	}

	ui.Emit(&EvtPullStart{Model: "llama3.2", Tag: "latest"})
	ui.Emit(&EvtManifestFetched{Model: "llama3.2", Tag: "latest", BlobCount: 5, TotalSize: 4700000000})
	ui.Emit(&EvtBlobCached{Blob: BlobRef{Digest: "sha256:34bb5ab01051abcdef1234567890ab", Size: 128000, Index: 1, Total: 5}})
	ui.Emit(&EvtBlobStart{Blob: BlobRef{Digest: "sha256:def456abc789abcdef1234567890ab", Size: 4500000000, Index: 2, Total: 5}})
	ui.Emit(&EvtChunkProgress{OverallDone: 3100000000, OverallTotal: 4700000000, BytesPerSecond: 8000000, ETASeconds: 492})
	ui.Emit(&EvtBlobVerifyStart{Blob: BlobRef{Digest: "sha256:def456abc789abcdef1234567890ab"}})
	ui.Emit(&EvtBlobVerified{Blob: BlobRef{Digest: "sha256:def456abc789abcdef1234567890ab"}, ElapsedMs: 754000})
	ui.Emit(&EvtBlobFinalized{Blob: BlobRef{Digest: "sha256:def456abc789abcdef1234567890ab"}})
	ui.Emit(&EvtManifestWriteStart{Model: "llama3.2", Tag: "latest"})
	ui.Emit(&EvtManifestWritten{Model: "llama3.2", Tag: "latest"})
	ui.Emit(&EvtPullCompleted{Model: "llama3.2", Tag: "latest", TotalBytes: 4700000000, ElapsedMs: 754000, BlobCount: 5})

	t.Logf("stdout:\n%s", ui.stdout.(*bytes.Buffer).String())
	t.Logf("stderr:\n%s", ui.stderr.(*bytes.Buffer).String())
}

func TestPrettyRendererNoColor(t *testing.T) {
	caps := TTYCaps{IsTTY: true, Color: false, Unicode: false, Overwrite: true, Width: 80, Height: 24}
	cfg := DefaultConfig()
	cfg.NoColor = true
	ui := &UI{
		cfg:     cfg,
		mode:    ModeTable,
		stdout:  &bytes.Buffer{},
		stderr:  &bytes.Buffer{},
		isatty:  true,
		caps:    caps,
		pretty:  NewPrettyRenderer(caps),
	}

	ui.Emit(&EvtPullStart{Model: "llama3.2", Tag: "latest"})
	ui.Emit(&EvtManifestFetched{Model: "llama3.2", Tag: "latest", BlobCount: 5, TotalSize: 4700000000})
	ui.Emit(&EvtPullCompleted{Model: "llama3.2", Tag: "latest", TotalBytes: 4700000000, ElapsedMs: 754000, BlobCount: 5})

	out := ui.stdout.(*bytes.Buffer).String()
	if len(out) == 0 {
		t.Fatal("expected output with no-color, got empty")
	}
	t.Logf("output:\n%s", out)
}

func TestPrettyListTable(t *testing.T) {
	caps := TTYCaps{IsTTY: true, Color: true, Unicode: true, Overwrite: true, Width: 120, Height: 40}
	cfg := DefaultConfig()
	ui := &UI{
		cfg:     cfg,
		mode:    ModeTable,
		stdout:  &bytes.Buffer{},
		stderr:  &bytes.Buffer{},
		isatty:  true,
		caps:    caps,
		pretty:  NewPrettyRenderer(caps),
	}

	ui.Emit(&EvtListResult{Rows: []ModelRow{
		{Name: "registry.ollama.ai/library/llama3.2:latest", Size: 4700000000, Digest: "sha256:34bb5ab01051abcdef1234567890ab"},
		{Name: "registry.ollama.ai/library/mistral:latest", Size: 4100000000, Digest: "sha256:abcdef1234567890abcdef12345678"},
	}})

	t.Logf("output:\n%s", ui.stdout.(*bytes.Buffer).String())
}

func TestPrettyShowCard(t *testing.T) {
	caps := TTYCaps{IsTTY: true, Color: true, Unicode: true, Overwrite: true, Width: 120, Height: 40}
	cfg := DefaultConfig()
	ui := &UI{
		cfg:     cfg,
		mode:    ModeTable,
		stdout:  &bytes.Buffer{},
		stderr:  &bytes.Buffer{},
		isatty:  true,
		caps:    caps,
		pretty:  NewPrettyRenderer(caps),
	}

	ui.Emit(&EvtShowResult{
		Name:          "llama3.2:latest",
		Family:        "llama",
		ParameterSize: "3B",
		Quantization:  "Q4_K_M",
		Size:          4700000000,
		Layers: []LayerRow{
			{MediaType: "application/vnd.ollama.image.model", Digest: "sha256:34bb5ab01051abcdef1234567890ab", Size: 4500000000},
			{MediaType: "application/vnd.ollama.image.template", Digest: "sha256:def456abc789abcdef1234567890ab", Size: 128000},
		},
	})

	t.Logf("output:\n%s", ui.stdout.(*bytes.Buffer).String())
}

func TestPrettyErrorBox(t *testing.T) {
	caps := TTYCaps{IsTTY: true, Color: true, Unicode: true, Overwrite: true, Width: 120, Height: 40}
	cfg := DefaultConfig()
	ui := &UI{
		cfg:     cfg,
		mode:    ModeTable,
		stdout:  &bytes.Buffer{},
		stderr:  &bytes.Buffer{},
		isatty:  true,
		caps:    caps,
		pretty:  NewPrettyRenderer(caps),
	}

	ui.Emit(&EvtPullStart{Model: "badmodel", Tag: "latest"})
	ui.Emit(&EvtPullFailed{Model: "badmodel", Tag: "latest", Class: ClassPermanent, Sentinel: "ErrNotFound", Message: "model not found in registry"})

	t.Logf("stdout:\n%s", ui.stdout.(*bytes.Buffer).String())
	t.Logf("stderr:\n%s", ui.stderr.(*bytes.Buffer).String())
}

func TestPrettyRmSequence(t *testing.T) {
	caps := TTYCaps{IsTTY: true, Color: true, Unicode: true, Overwrite: true, Width: 120, Height: 40}
	cfg := DefaultConfig()
	ui := &UI{
		cfg:     cfg,
		mode:    ModeTable,
		stdout:  &bytes.Buffer{},
		stderr:  &bytes.Buffer{},
		isatty:  true,
		caps:    caps,
		pretty:  NewPrettyRenderer(caps),
	}

	ui.Emit(&EvtRmStart{Model: "llama3.2:latest"})
	ui.Emit(&EvtRmBlobDeleted{Digest: "sha256:34bb5ab01051abcdef1234567890ab", Size: 4500000000})
	ui.Emit(&EvtRmManifestDeleted{Model: "llama3.2", Tag: "latest"})
	ui.Emit(&EvtRmCompleted{Model: "llama3.2:latest", BlobsDeleted: 1, BytesReclaimed: 4500000000})

	t.Logf("output:\n%s", ui.stdout.(*bytes.Buffer).String())
}

func TestProgressBar(t *testing.T) {
	caps := TTYCaps{IsTTY: true, Color: true, Unicode: true, Overwrite: true, Width: 120, Height: 40}
	bar := renderBar(caps, 67, 4700000000, 3100000000, 8000000, 492)
	if len(bar) == 0 {
		t.Fatal("expected progress bar output")
	}
	t.Logf("bar: %s", bar)
}

func TestVisualLen(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"hello", 5},
		{FgBrand + "hello" + Reset, 5},
		{Bold + "hi" + Reset, 2},
	}
	for _, tt := range tests {
		got := visualLen(tt.input)
		if got != tt.want {
			t.Errorf("visualLen(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}