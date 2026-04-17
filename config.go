package main

import (
	"os"
	"path/filepath"
	"time"
)

type Config struct {
	ModelsDir   string
	Scheme      string        // "https" | "http"
	MaxRetries  int           // 6
	ChunkSize   int64         // 64 MiB (single-chunk fallback only)
	Timeout     time.Duration // 30m per chunk
	Quiet       bool
	Verbose     bool
	NoColor     bool
	Concurrency int           // always 1 in v1; not exposed as a flag
	RegistryURL string       // derived
}

func DefaultConfig() *Config {
	modelsDir := os.Getenv("OLLAMA_MODELS")
	if modelsDir == "" {
		home, _ := os.UserHomeDir()
		modelsDir = filepath.Join(home, ".ollama", "models")
	}
	return &Config{
		ModelsDir:   modelsDir,
		Scheme:      "https",
		MaxRetries:  6,
		ChunkSize:   64 * 1024 * 1024, // 64 MiB
		Timeout:     30 * time.Minute,
		Concurrency: 1,
	}
}