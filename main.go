package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func PullModel(ctx context.Context, cfg *Config, ref ModelRef, ui *UI) error {
	ui.Emit(&EvtPullStart{Model: ref.Name, Tag: ref.Tag})
	ui.Emit(&EvtManifestFetchStart{Model: ref.Name, Tag: ref.Tag})

	manifest, err := FetchManifest(ctx, cfg, ref)
	if err != nil {
		ui.Emit(&EvtPullFailed{Model: ref.Name, Tag: ref.Tag, Class: classifyErr(err), Sentinel: sentinalName(err), Message: err.Error()})
		return err
	}

	allLayers := append([]ManifestLayer{manifest.Config}, manifest.Layers...)
	ui.Emit(&EvtManifestFetched{Model: ref.Name, Tag: ref.Tag, BlobCount: len(allLayers), TotalSize: totalSize(allLayers)})

	var overallDone int64
	for i, layer := range allLayers {
		if err := DownloadBlob(ctx, cfg, layer, ref, ui, i+1, len(allLayers), overallDone, totalSize(allLayers)); err != nil {
			ui.Emit(&EvtPullFailed{Model: ref.Name, Tag: ref.Tag, Class: classifyErr(err), Sentinel: sentinalName(err), Message: err.Error()})
			return err
		}
		overallDone += layer.Size
	}

	blobsDir := filepath.Join(cfg.ModelsDir, "blobs")
	for _, layer := range allLayers {
		dn := strings.Replace(layer.Digest, ":", "-", 1)
		if _, err := os.Stat(filepath.Join(blobsDir, dn)); err != nil {
			return fmt.Errorf("blob %s missing after download: %v", layer.Digest, err)
		}
	}

	ui.Emit(&EvtManifestWriteStart{Model: ref.Name, Tag: ref.Tag})
	data, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	if err := atomicWrite(filepath.Join(cfg.ModelsDir, "manifests", ref.ManifestPath()), data); err != nil {
		return err
	}
	ui.Emit(&EvtManifestWritten{Model: ref.Name, Tag: ref.Tag})
	return nil
}

func totalSize(layers []ManifestLayer) int64 {
	var s int64
	for _, l := range layers {
		s += l.Size
	}
	return s
}

func classifyErr(err error) ErrorClass {
	cls := ClassifyError(err)
	switch cls {
	case ClassTransient:
		return ClassTransient
	case ClassPermanent:
		return ClassPermanent
	default:
		return ClassCorrupt
	}
}

func sentinalName(err error) string {
	switch err {
	case ErrNotFound:
		return "ErrNotFound"
	case ErrAuthFailed:
		return "ErrAuthFailed"
	case ErrDiskFull:
		return "ErrDiskFull"
	case ErrChunkMismatch:
		return "ErrChunkMismatch"
	case ErrBlobMismatch:
		return "ErrBlobMismatch"
	case ErrCheckpointCorrupt:
		return "ErrCheckpointCorrupt"
	case ErrManifestCorrupt:
		return "ErrManifestCorrupt"
	}
	return ""
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func printHelp(w *os.File) {
	fmt.Fprintln(w, `Pullama — A resumable model puller for Ollama
		By Core-Fidelity

Usage:
  pullama <model> [flags]         Pull a model
  pullama list                     List installed models
  pullama show <model>             Show model details
  pullama rm <model>               Remove a model (shared-blob aware)
  pullama clean                    Clean partial downloads, locks, checkpoints
  pullama queue <add|list|rm|start> Manage download queue
  pullama version                  Print version
  pullama help                     Show this help

Queue subcommands:
  pullama queue add <model> [...]  Add models to the queue
  pullama queue list               Show the queue
  pullama queue rm <number>        Remove a queue entry
  pullama queue start              Process the queue

Flags:
  --insecure          Use http:// instead of https://
  --models-dir <dir>  Storage root (default $OLLAMA_MODELS or ~/.ollama/models)
  --quiet             Summary only, no progress bar
  --verbose           Checkpoint saves, HTTP details, chunk boundaries
  --no-color          Strip ANSI colors and Unicode box-drawing
  --output <mode>     Output mode: table (default), compact, json, debug
  --max-retries <n>   Max transient retries per chunk (default 6)
  --chunk-size <n>    Chunk size when server provides no chunksums (default 64 MiB)
  --timeout <dur>     Per-chunk HTTP timeout (default 30m)

Exit codes:
  0  Success
  1  General error
  2  Disk full
  3  Authentication failed
  4  Model not found`)
}

func runQueue(args []string, cfg *Config, ui *UI) {
	if len(args) < 1 {
		die("usage: pullama queue <add|list|rm|start> [args]")
	}
	switch args[0] {
	case "add":
		if len(args) < 2 {
			die("usage: pullama queue add <model> [model ...]")
		}
		var refs []ModelRef
		for _, a := range args[1:] {
			ref, err := ParseModelRef(a)
			if err != nil {
				die("invalid model %q: %v", a, err)
			}
			refs = append(refs, ref)
		}
		q, err := LoadQueue(cfg.ModelsDir)
		if err != nil {
			die("error: %v", err)
		}
		added, err := q.Add(refs)
		if err != nil {
			die("error: %v", err)
		}
		if added > 0 {
			fmt.Printf("added %d model(s) to queue\n", added)
		} else {
			fmt.Println("all models already in queue")
		}
	case "list":
		q, err := LoadQueue(cfg.ModelsDir)
		if err != nil {
			die("error: %v", err)
		}
		if len(q.Entries) == 0 {
			fmt.Println("queue is empty")
			return
		}
		for i, e := range q.Entries {
			icon := " "
			switch e.Status {
			case QueuedStatus:
				icon = "·"
			case ActiveStatus:
				icon = "›"
			case CompletedStatus:
				icon = "✓"
			case FailedStatus:
				icon = "✗"
			}
			errStr := ""
			if e.Error != "" {
				errStr = " — " + e.Error
			}
			fmt.Printf("  %s %d  %s:%s  %s%s\n", icon, i+1, e.Model, e.Tag, e.Status, errStr)
		}
	case "rm":
		if len(args) < 2 {
			die("usage: pullama queue rm <number>")
		}
		var idx int
		if _, err := fmt.Sscanf(args[1], "%d", &idx); err != nil || idx < 1 {
			die("invalid index %q", args[1])
		}
		q, err := LoadQueue(cfg.ModelsDir)
		if err != nil {
			die("error: %v", err)
		}
		if err := q.Remove(idx - 1); err != nil {
			die("error: %v", err)
		}
		fmt.Printf("removed entry %d\n", idx)
	case "start":
		if err := RunQueue(context.Background(), cfg, ui); err != nil {
			if err == context.Canceled {
				os.Exit(0)
			}
			die("error: %v", err)
		}
	default:
		die("unknown queue subcommand %q (add, list, rm, start)", args[0])
	}
}

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printHelp(os.Stderr)
		os.Exit(1)
	}

	// Handle -h/--help anywhere
	for _, a := range os.Args[1:] {
		if a == "-h" || a == "--help" {
			printHelp(os.Stdout)
			os.Exit(0)
		}
	}
	cfg := DefaultConfig()
	ui := NewUI(cfg)
	defer ui.Teardown()

	switch os.Args[1] {
	case "help", "-h", "--help":
		printHelp(os.Stdout)
	case "version", "-v", "--version":
		fmt.Println(version)
	case "queue":
		runQueue(os.Args[2:], cfg, ui)
	case "list":
		models, err := ListModels(cfg.ModelsDir)
		if err != nil {
			die("error: %v", err)
		}
		var rows []ModelRow
		for _, m := range models {
			rows = append(rows, ModelRow{Name: m.Name, Size: m.Size, Modified: m.Modified})
		}
		ui.Emit(&EvtListResult{Rows: rows})
	case "show":
		if len(os.Args) < 3 {
			die("usage: pullama show <model>")
		}
		d, err := ShowModel(os.Args[2], cfg.ModelsDir)
		if err != nil {
			die("error: %v", err)
		}
		ui.Emit(&EvtShowResult{Name: d.Name, Family: d.Family, ParameterSize: d.ParameterSize, Quantization: d.Quantization, Size: d.Size, Config: d.Layers[0], Layers: d.Layers[1:]})
	case "rm":
		if len(os.Args) < 3 {
			die("usage: pullama rm <model>")
		}
		if err := RemoveModel(os.Args[2], cfg.ModelsDir, ui); err != nil {
			die("error: %v", err)
		}
	case "clean":
		n, err := Clean(cfg.ModelsDir, ui)
		if err != nil {
			die("error: %v", err)
		}
		_ = n
	default:
		ref, err := ParseModelRef(os.Args[1])
		if err != nil {
			die("error: %v", err)
		}
		for i := 2; i < len(os.Args); i++ {
			switch os.Args[i] {
			case "--insecure":
				cfg.Scheme = "http"
			case "--models-dir":
				i++
				cfg.ModelsDir = os.Args[i]
			case "--quiet":
				cfg.Quiet = true
			case "--verbose":
				cfg.Verbose = true
			case "--no-color":
				cfg.NoColor = true
			case "--max-retries":
				i++
				fmt.Sscanf(os.Args[i], "%d", &cfg.MaxRetries)
			case "--chunk-size":
				i++
				fmt.Sscanf(os.Args[i], "%d", &cfg.ChunkSize)
			case "--timeout":
				i++
				cfg.Timeout, _ = time.ParseDuration(os.Args[i])
			case "--output":
				i++
				switch os.Args[i] {
				case "json":
					ui.mode = ModeJSON
				case "compact":
					ui.mode = ModeCompact
				case "debug":
					ui.mode = ModeDebug
				}
			}
		}
		start := time.Now()
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()
		if err := PullModel(ctx, cfg, ref, ui); err != nil {
			elapsed := time.Since(start).Milliseconds()
			ui.Emit(&EvtPullFailed{Model: ref.Name, Tag: ref.Tag, Class: classifyErr(err), Sentinel: sentinalName(err), Message: err.Error()})
			_ = elapsed
			switch {
			case err == ErrDiskFull:
				os.Exit(2)
			case err == ErrAuthFailed:
				os.Exit(3)
			case err == ErrNotFound:
				os.Exit(4)
			default:
				os.Exit(1)
			}
		}
		elapsed := time.Since(start).Milliseconds()
		layers := []ManifestLayer{} // approximate; we don't have manifest here
		ui.Emit(&EvtPullCompleted{Model: ref.Name, Tag: ref.Tag, ElapsedMs: elapsed, BlobCount: len(layers)})
	}
}
