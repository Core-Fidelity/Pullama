package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// Queue model — persisted in ~/.ollama/models/.pullm/queue.json
// ─────────────────────────────────────────────────────────────────────────────

type QueueStatus string

const (
	QueuedStatus    QueueStatus = "queued"
	ActiveStatus    QueueStatus = "active"
	CompletedStatus QueueStatus = "completed"
	FailedStatus    QueueStatus = "failed"
)

type QueueEntry struct {
	Model    string     `json:"model"`
	Tag      string     `json:"tag"`
	Status   QueueStatus `json:"status"`
	AddedAt  time.Time  `json:"added_at"`
	Finished time.Time  `json:"finished_at,omitempty"`
	Error    string     `json:"error,omitempty"`
}

type Queue struct {
	mu      sync.Mutex
	path    string
	Entries []QueueEntry `json:"entries"`
}

func queuePath(modelsDir string) string {
	return filepath.Join(modelsDir, ".pullm", "queue.json")
}

func LoadQueue(modelsDir string) (*Queue, error) {
	path := queuePath(modelsDir)
	q := &Queue{path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return q, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return q, nil
	}
	if err := json.Unmarshal(data, q); err != nil {
		return nil, err
	}
	return q, nil
}

func (q *Queue) save() error {
	data, err := json.MarshalIndent(q, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(q.path, data)
}

// Add appends models to the queue. Skips duplicates already in any non-failed state.
func (q *Queue) Add(models []ModelRef) (added int, err error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	existing := map[string]bool{}
	for _, e := range q.Entries {
		key := e.Model + ":" + e.Tag
		if e.Status != FailedStatus {
			existing[key] = true
		}
	}

	for _, ref := range models {
		key := ref.Name + ":" + ref.Tag
		if existing[key] {
			continue
		}
		q.Entries = append(q.Entries, QueueEntry{
			Model:   ref.Name,
			Tag:     ref.Tag,
			Status:  QueuedStatus,
			AddedAt: time.Now(),
		})
		added++
		existing[key] = true
	}
	if added > 0 {
		err = q.save()
	}
	return
}

// Remove removes the entry at the given index. Returns an error if index is out of bounds.
// The active entry (status=active) cannot be removed — it must be cancelled via the running process.
func (q *Queue) Remove(index int) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if index < 0 || index >= len(q.Entries) {
		return fmt.Errorf("queue index %d out of range (0-%d)", index, len(q.Entries)-1)
	}
	if q.Entries[index].Status == ActiveStatus {
		return fmt.Errorf("cannot remove active entry; use Ctrl+C to cancel the running pull")
	}
	q.Entries = append(q.Entries[:index], q.Entries[index+1:]...)
	return q.save()
}

// NextPending returns the first queued entry and marks it active, or nil if none.
func (q *Queue) NextPending() *QueueEntry {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i := range q.Entries {
		if q.Entries[i].Status == QueuedStatus {
			q.Entries[i].Status = ActiveStatus
			q.save()
			return &q.Entries[i]
		}
	}
	return nil
}

// MarkCompleted marks the entry with the given model:tag as completed.
func (q *Queue) MarkCompleted(model, tag string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i := range q.Entries {
		if q.Entries[i].Model == model && q.Entries[i].Tag == tag && q.Entries[i].Status == ActiveStatus {
			q.Entries[i].Status = CompletedStatus
			q.Entries[i].Finished = time.Now()
			q.save()
			return
		}
	}
}

// MarkFailed marks the entry with the given model:tag as failed with the error message.
func (q *Queue) MarkFailed(model, tag, errMsg string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i := range q.Entries {
		if q.Entries[i].Model == model && q.Entries[i].Tag == tag && q.Entries[i].Status == ActiveStatus {
			q.Entries[i].Status = FailedStatus
			q.Entries[i].Error = errMsg
			q.Entries[i].Finished = time.Now()
			q.save()
			return
		}
	}
}

// Prune removes completed and failed entries, returning the number removed.
func (q *Queue) Prune() (int, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	n := 0
	j := 0
	for _, e := range q.Entries {
		if e.Status == CompletedStatus || e.Status == FailedStatus {
			n++
			continue
		}
		q.Entries[j] = e
		j++
	}
	q.Entries = q.Entries[:j]
	if n > 0 {
		return n, q.save()
	}
	return 0, nil
}