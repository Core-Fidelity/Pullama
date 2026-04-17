package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestQueueAddDedup(t *testing.T) {
	dir := t.TempDir()
	modelsDir := filepath.Join(dir, "models")
	os.MkdirAll(filepath.Join(modelsDir, ".pullm"), 0o755)

	q, err := LoadQueue(modelsDir)
	if err != nil {
		t.Fatal(err)
	}

	added, _ := q.Add([]ModelRef{
		{Name: "llama3.2", Tag: "latest"},
		{Name: "mistral", Tag: "7b"},
	})
	if added != 2 {
		t.Fatalf("expected 2 added, got %d", added)
	}
	if len(q.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(q.Entries))
	}

	// Add duplicate
	added, _ = q.Add([]ModelRef{
		{Name: "llama3.2", Tag: "latest"},
	})
	if added != 0 {
		t.Fatalf("duplicate should not be added, got %d", added)
	}
	if len(q.Entries) != 2 {
		t.Fatalf("expected 2 entries after dedup, got %d", len(q.Entries))
	}
}

func TestQueueRemove(t *testing.T) {
	dir := t.TempDir()
	modelsDir := filepath.Join(dir, "models")
	os.MkdirAll(filepath.Join(modelsDir, ".pullm"), 0o755)

	q, _ := LoadQueue(modelsDir)
	q.Add([]ModelRef{
		{Name: "a", Tag: "1"},
		{Name: "b", Tag: "2"},
		{Name: "c", Tag: "3"},
	})

	if err := q.Remove(1); err != nil {
		t.Fatal(err)
	}
	if len(q.Entries) != 2 {
		t.Fatalf("expected 2 entries after remove, got %d", len(q.Entries))
	}
	if q.Entries[0].Model != "a" || q.Entries[1].Model != "c" {
		t.Fatalf("wrong entries after remove: %v", q.Entries)
	}
}

func TestQueueNextPending(t *testing.T) {
	dir := t.TempDir()
	modelsDir := filepath.Join(dir, "models")
	os.MkdirAll(filepath.Join(modelsDir, ".pullm"), 0o755)

	q, _ := LoadQueue(modelsDir)
	q.Add([]ModelRef{
		{Name: "a", Tag: "1"},
		{Name: "b", Tag: "2"},
	})

	e := q.NextPending()
	if e == nil || e.Model != "a" || e.Status != ActiveStatus {
		t.Fatalf("expected first entry active, got %v", e)
	}

	// NextPending should skip the active one and return the next queued
	e = q.NextPending()
	if e == nil || e.Model != "b" || e.Status != ActiveStatus {
		t.Fatalf("expected second entry active, got %v", e)
	}

	// No more pending
	e = q.NextPending()
	if e != nil {
		t.Fatalf("expected nil, got %v", e)
	}
}

func TestQueueMarkCompletedFailed(t *testing.T) {
	dir := t.TempDir()
	modelsDir := filepath.Join(dir, "models")
	os.MkdirAll(filepath.Join(modelsDir, ".pullm"), 0o755)

	q, _ := LoadQueue(modelsDir)
	q.Add([]ModelRef{
		{Name: "a", Tag: "1"},
		{Name: "b", Tag: "2"},
	})

	q.NextPending()
	q.MarkCompleted("a", "1")
	q.NextPending()
	q.MarkFailed("b", "2", "not found")

	if q.Entries[0].Status != CompletedStatus {
		t.Fatalf("expected completed, got %s", q.Entries[0].Status)
	}
	if q.Entries[1].Status != FailedStatus || q.Entries[1].Error != "not found" {
		t.Fatalf("expected failed with error, got %s: %s", q.Entries[1].Status, q.Entries[1].Error)
	}
}

func TestQueuePersistence(t *testing.T) {
	dir := t.TempDir()
	modelsDir := filepath.Join(dir, "models")
	os.MkdirAll(filepath.Join(modelsDir, ".pullm"), 0o755)

	q1, _ := LoadQueue(modelsDir)
	q1.Add([]ModelRef{{Name: "test", Tag: "1"}})

	// Reload from disk
	q2, err := LoadQueue(modelsDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(q2.Entries) != 1 {
		t.Fatalf("expected 1 persisted entry, got %d", len(q2.Entries))
	}
	if q2.Entries[0].Model != "test" || q2.Entries[0].Tag != "1" {
		t.Fatalf("wrong persisted entry: %v", q2.Entries[0])
	}
}