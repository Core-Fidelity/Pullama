# Core-Fidelity presents Pullama.

Resumable model puller for [Ollama](https://ollama.ai).

Downloads models directly from the Ollama registry with crash-safe resume. Works over unreliable connections — kill it, reboot, run it again, and it picks up exactly where it left off.

No Ollama server required. Single static binary. Zero runtime dependencies.

## Install

Download a release binary, or build from source:

```bash
go build -trimpath -ldflags="-buildid=" -o pullama .
```

Put it on your PATH (e.g. `~/.local/bin`):

```bash
mkdir -p ~/.local/bin
cp pullama ~/.local/bin/
```

Cross-compile:

```bash
GOOS=darwin  GOARCH=arm64 go build -trimpath -ldflags="-buildid=" -o pullama-darwin-arm64 .
GOOS=linux   GOARCH=amd64 go build -trimpath -ldflags="-buildid=" -o pullama-linux-amd64 .
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-buildid=" -o pullama-windows-amd64.exe .
```

## Usage

```bash
# Pull a model
pullama llama3.2
pullama mistral:7b
pullama user/my-model

# Override storage location
pullama llama3.2 --models-dir /data/ollama

# Use plain HTTP (for local registries)
pullama my-model --insecure

# Output modes
pullama llama3.2 --output json      # structured JSON events
pullama llama3.2 --output compact   # minimal one-line updates
pullama llama3.2 --output debug     # verbose Go struct output

# Quiet mode (one summary line)
pullama llama3.2 --quiet

# Verbose mode (checkpoint saves, HTTP details, chunk boundaries)
pullama llama3.2 --verbose

# List locally installed models
pullama list

# Show model details (family, parameters, quantization, layers)
pullama show llama3.2

# Remove a model (shared-blob aware — won't delete blobs used by other models)
pullama rm llama3.2

# Clean up disposable artifacts (partial downloads, locks, checkpoints)
pullama clean
```

## Queue

Queue multiple models for sequential download. Failed models are marked and skipped — one bad model doesn't block the rest.

```bash
# Add models to the queue
pullama queue add llama3.2 mistral:7b phi3:mini
# added 3 model(s) to queue

# List the queue
pullama queue list
#   · 1  llama3.2:latest  queued
#   · 2  mistral:7b        queued
#   · 3  phi3:mini          queued

# Remove an entry (by position number)
pullama queue rm 2

# Start processing the queue
pullama queue start
# ▸ queue [1/2] pulling llama3.2:latest
#   ... (normal pull output) ...
# ▸ queue [1/2] ✓ completed llama3.2:latest
# ▸ queue [2/2] pulling phi3:mini
#   ...
```

Duplicates are skipped — adding a model that's already queued or active does nothing. Only one `pullama queue start` can run at a time (queue-level lock). Ctrl+C stops after the current model finishes and the queue remains paused — run `pullama queue start` again to continue.

Active entries can't be removed from the queue (cancel the running process instead). Pending entries can be removed freely.

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--insecure` | off | Use `http://` instead of `https://` |
| `--models-dir` | `$OLLAMA_MODELS` or `~/.ollama/models` | Storage root |
| `--quiet` | off | Suppress progress bar; emit final summary only |
| `--verbose` | off | Log checkpoint saves, HTTP details, chunk boundaries |
| `--no-color` | off | Strip ANSI colors and Unicode box-drawing |
| `--output` | `table` | Output mode: `table`, `compact`, `json`, `debug` |
| `--max-retries` | 6 | Max transient retries per chunk |
| `--chunk-size` | 64 MiB | Chunk size when server doesn't provide chunksums |
| `--timeout` | 30m | Per-chunk HTTP timeout |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Disk full — try `pullama clean` |
| 3 | Authentication failed — check `~/.ollama/id_ed25519` |
| 4 | Model not found |

## How It Works

pullama talks directly to the Ollama registry (the same one `ollama pull` uses). It authenticates with the same Ed25519 key at `~/.ollama/id_ed25519` and writes blobs to the same `~/.ollama/models/blobs/` directory. Models pulled with pullama appear in `ollama list` without any extra steps.

### Crash-safe resume

Every download writes persistent checkpoints to disk. If the process is killed (SIGINT, SIGKILL, kernel panic), a subsequent run:

1. Reopens the `.partial` file
2. Validates the checkpoint against the manifest
3. Re-verifies the last chunk that was written (catches torn writes)
4. Truncates any unverified data
5. Resumes from the exact byte boundary

No re-downloading of already-verified data. Full-blob SHA256 verification runs before every `.partial`-to-final rename.

### File layout

Files written to `~/.ollama/models/`:

```
blobs/sha256-<hex>             # final verified blobs (shared with Ollama)
blobs/sha256-<hex>.partial     # in-progress download data
blobs/sha256-<hex>.lock        # OS advisory lock (auto-released on crash)
.pullm/sha256-<hex>.json       # download checkpoint
.pullm/queue.json              # download queue (pullama queue)
manifests/<host>/<ns>/<model>/<tag>  # model manifest (written last)
```

Partial files, locks, checkpoints, and queue state are disposable — deleting them is always safe (worst case: download restarts from offset 0, queue is lost). Final blobs and manifests are never modified after write.

### Atomicity

All state transitions follow the same pattern:

```
write path.tmp → fsync(path.tmp) → rename(path.tmp, path) → fsync(parent_dir)
```

A crash at any point leaves the previous valid state intact.

### Concurrency

One blob at a time, one chunk at a time. OS advisory locks (`flock` on Unix, `LockFileEx` on Windows) prevent two pullama processes from writing the same `.partial`. Locks are released by the kernel on process exit — no stale-lock issues.

## Signal Handling

**SIGINT / SIGTERM** — graceful shutdown:

- The current chunk finishes its write and hash verification
- If verified, the checkpoint is saved; if not, unverified data is truncated
- The lock is released
- Prints a resume hint: `interrupted — resume with: pullama <model>`
- Exits 0

A second signal exits immediately with code 1.

## Retry Behavior

| Condition | Action | Limit |
|-----------|--------|-------|
| Connection reset / timeout / DNS / 5xx / 429 | Exponential backoff retry (0.5s–120s ± jitter) | 6 retries per chunk |
| 401 from registry | Regenerate auth token | 3 refreshes per blob |
| 403 from CDN | Re-resolve blob URL | 5 refreshes per blob |
| Chunk hash mismatch | Truncate to verified boundary, retry | 6 retries per chunk |
| Full-blob hash mismatch | Delete .partial + checkpoint, re-download | 2 full re-downloads |

## `pullama rm` — safe deletion

Removal is shared-blob aware:

1. Acquires a directory lock on `manifests/`
2. Reads the target manifest
3. Scans **all** other manifests to build the active-digest set
4. If any other manifest fails to parse, the entire deletion **aborts** — no files are removed
5. Deletes only blobs referenced exclusively by the target model
6. Prunes empty parent directories

## `pullama clean` — disposable cleanup

Removes `.pullm/*.json` checkpoints, `blobs/*.partial`, and `blobs/*.lock`. Never touches final blobs or manifests. Safe to run at any time. Idempotent.

## Output Modes

### Table (default — pretty TTY output)

```
  ▸ core-fidelity - pullama
╭─ pulling llama3.2:latest ─────────────────────╮
│  ✓ manifest · 5 blobs · 4.4 GB
│  ◆ cached    34bb5ab01051 125.0 kB [1/5]
│  › downloading def456abc789 4.2 GB [2/5]
│  [███████████████████▌░░░░░░░░░] 67% 2.9 GB/4.4 GB · 8.2 MB/s · eta 2m30s
│  ✓ verified  def456abc789 (12m34s)
│  ✓ finalized def456abc789
│  ✓ manifest written
╰─────────────────────────────────────────────╯
╔═════════════════════════════════════════════╗
║  ✓  pulled llama3.2:latest                  ║
╠═════════════════════════════════════════════╣
║    size      4.4 GB                         ║
║    blobs     5                              ║
║    elapsed   12m34s                         ║
║    avg rate  5.9 MB/s                       ║
╚═════════════════════════════════════════════╝
```

Non-TTY output (piped, CI, `TERM=dumb`) automatically strips colors, Unicode, and spinners.

### Compact

```
pulling llama3.2:latest
67% 3.1/4.7 GB
completed 4.7 GB 12m34s
```

### JSON

Each event is a JSON line — useful for piping to `jq` or programmatic consumption:

```json
{"Model":"llama3.2","Tag":"latest"}
{"BlobCount":5,"TotalSize":4831838208}
{"pct":67,"OverallDone":3145728000,"OverallTotal":4831838208}
```

### Debug

Raw Go struct formatting of every event — for development and debugging.

## Authentication

pullama uses the same Ed25519 key as Ollama (`~/.ollama/id_ed25519`). If you've used `ollama pull` or `ollama run` on this machine, the key already exists and pullama will use it. If not, you'll need to generate one or copy it from a machine that has one.

## Compatibility

- Writes files that `ollama list` reads natively
- Coexists with a running Ollama server — blobs are content-addressed, so concurrent writes are safe
- Works on macOS (arm64/amd64), Linux (amd64/arm64), and Windows (amd64)

## Support

If Pullama saved you time (or bandwidth), consider supporting development:

- [Buy me a coffee](https://buymeacoffee.com/corefidelity)
- [GitHub Sponsors](https://github.com/sponsors/Core-Fidelity)

## License

MIT