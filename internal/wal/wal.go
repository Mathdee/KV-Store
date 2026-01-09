package wal

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

type pendingWrite struct {
	entry string
	done  chan error
}

type WAL struct {
	file *os.File
	mu   sync.Mutex

	// Group commit
	pending     []pendingWrite
	pendingMu   sync.Mutex
	flushTicker *time.Ticker
	closeCh     chan struct{}
}

func NewWAL(filename string) (*WAL, error) {
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	w := &WAL{
		file:        f,
		pending:     make([]pendingWrite, 0, 1000),
		flushTicker: time.NewTicker(5 * time.Millisecond), // Flush every 5ms
		closeCh:     make(chan struct{}),
	}

	// Start background flusher
	go w.flushLoop()

	return w, nil
}

// flushLoop runs in background, batching writes
func (w *WAL) flushLoop() {
	for {
		select {
		case <-w.flushTicker.C:
			w.flush()
		case <-w.closeCh:
			w.flush() // Final flush before close
			return
		}
	}
}

// flush writes all pending entries in ONE fsync
func (w *WAL) flush() {
	w.pendingMu.Lock()
	if len(w.pending) == 0 {
		w.pendingMu.Unlock()
		return
	}

	// Grab all pending writes
	toFlush := w.pending
	w.pending = make([]pendingWrite, 0, 1000)
	w.pendingMu.Unlock()

	// Write all entries to file (one syscall per entry, but no sync yet)
	w.mu.Lock()
	var writeErr error
	for _, pw := range toFlush {
		if _, err := w.file.WriteString(pw.entry); err != nil {
			writeErr = err
			break
		}
	}

	// ONE fsync for ALL entries
	if writeErr == nil {
		writeErr = w.file.Sync()
	}
	w.mu.Unlock()

	// Notify all waiting goroutines
	for _, pw := range toFlush {
		pw.done <- writeErr
		close(pw.done)
	}
}

// WriteEntry queues a write and waits for group commit
func (w *WAL) WriteEntry(key, value string) error {
	entry := fmt.Sprintf("%s,%s\n", key, value)
	done := make(chan error, 1)

	// Add to pending batch
	w.pendingMu.Lock()
	w.pending = append(w.pending, pendingWrite{entry: entry, done: done})
	w.pendingMu.Unlock()

	// Wait for flush
	return <-done
}

func (w *WAL) Close() error {
	close(w.closeCh)
	w.flushTicker.Stop()
	return w.file.Close()
}

func Recover(filename string) (map[string]string, error) {
	data := make(map[string]string)

	f, err := os.Open(filename)
	if os.IsNotExist(err) {
		return data, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, ",")
		if len(parts) == 2 {
			data[parts[0]] = parts[1]
		}
	}
	return data, nil
}
