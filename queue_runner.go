package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"
)

// RunQueue processes all queued entries sequentially. It acquires the queue-level
// lock, iterates pending entries, and runs PullModel for each. Failed entries
// are marked and skipped. The queue-level lock prevents concurrent runners.
func RunQueue(ctx context.Context, cfg *Config, ui *UI) error {
	q, err := LoadQueue(cfg.ModelsDir)
	if err != nil {
		return fmt.Errorf("loading queue: %w", err)
	}

	// Acquire queue lock
	lockPath := queueLockPath(cfg.ModelsDir)
	lock, err := AcquireLock(lockPath)
	if err != nil {
		if err == ErrLockBusy {
			return fmt.Errorf("another pullama queue runner is already active")
		}
		return err
	}
	defer lock.Release()

	// Count pending
	total := 0
	failed := 0
	for _, e := range q.Entries {
		if e.Status == QueuedStatus || e.Status == ActiveStatus {
			total++
		}
	}
	if total == 0 {
		ui.Emit(&EvtQueueStart{Index: 0, Total: 0})
		return nil
	}

	// Set up signal handling
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	idx := 0
	for {
		entry := q.NextPending()
		if entry == nil {
			break
		}

		ui.Emit(&EvtQueueStart{Index: idx, Total: total, Model: entry.Model, Tag: entry.Tag})

		ref, err := ParseModelRef(entry.Model + ":" + entry.Tag)
		if err != nil {
			q.MarkFailed(entry.Model, entry.Tag, err.Error())
			failed++
			ui.Emit(&EvtQueueFailed{Index: idx, Total: total, Model: entry.Model, Tag: entry.Tag, Error: err.Error(), Failed: failed})
			idx++
			continue
		}

		err = PullModel(ctx, cfg, ref, ui)
		if err != nil {
			q.MarkFailed(entry.Model, entry.Tag, err.Error())
			failed++
			ui.Emit(&EvtQueueFailed{Index: idx, Total: total, Model: entry.Model, Tag: entry.Tag, Error: err.Error(), Failed: failed})
			// If context was cancelled, stop the queue
			if ctx.Err() != nil {
				return ctx.Err()
			}
		} else {
			q.MarkCompleted(entry.Model, entry.Tag)
			ui.Emit(&EvtQueueCompleted{Index: idx, Total: total, Model: entry.Model, Tag: entry.Tag, Failed: failed})
		}
		idx++
	}

	return nil
}

func queueLockPath(modelsDir string) string {
	return fmt.Sprintf("%s/.pullm/queue.lock", modelsDir)
}