// Package testutil contains helpers shared by the tests: throw-away databases, a fake clock, and small test fakes.
package testutil

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"shopflow/internal/db"
	"shopflow/internal/todo"
)

func dbDir() string {
	_, file, _, _ := runtime.Caller(0) // .../internal/testutil/testutil.go
	return filepath.Join(filepath.Dir(file), "..", "..", "db")
}

// NewDB returns a brand-new SQLite database (schema + seed data) in a temp directory. Every test gets its own, so tests
// can never interfere with each other. It is closed and deleted automatically when the test ends.
func NewDB(t testing.TB) *sql.DB { return newDB(t, true) }

// NewEmptyDB is like NewDB but with the schema only (no rows).
func NewEmptyDB(t testing.TB) *sql.DB { return newDB(t, false) }

func newDB(t testing.TB, seed bool) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "shopflow-test.db")
	d, err := db.Open(path)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	seedPath := ""
	if seed {
		seedPath = filepath.Join(dbDir(), "seed.sql")
	}
	if err := db.Initialize(context.Background(), d, filepath.Join(dbDir(), "schema.sql"), seedPath); err != nil {
		t.Fatalf("initialize test db: %v", err)
	}
	return d
}

// Scalar runs a query that returns one integer: handy for asserting on database state independent of your repositories.
func Scalar(t testing.TB, d *sql.DB, query string, args ...any) int64 {
	t.Helper()
	var v int64
	if err := d.QueryRow(query, args...).Scan(&v); err != nil {
		t.Fatalf("scalar %q: %v", query, err)
	}
	return v
}

// Exec runs a statement and fails the test on error.
func Exec(t testing.TB, d *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := d.Exec(query, args...); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

// ExpectError fails the test unless err is a REAL error. An unfinished stub's "not implemented" error does not count:
// otherwise "this should fail" tests would pass before you had written anything.
func ExpectError(t testing.TB, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if errors.Is(err, todo.ErrNotImplemented) {
		t.Fatalf("expected a real error from your code, but this is still the unfinished stub: %v", err)
	}
}

// Receive waits for a value from ch, failing the test (instead of hanging forever) after d.
func Receive[T any](t testing.TB, ch <-chan T, d time.Duration) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(d):
		t.Fatalf("timed out after %v waiting for a value", d)
		panic("unreachable")
	}
}

// WaitClosed waits for ch to be closed, failing the test after d.
func WaitClosed(t testing.TB, ch <-chan struct{}, d time.Duration) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(d):
		t.Fatalf("timed out after %v waiting for a channel to close", d)
	}
}

// Timeout is the upper bound for "this should finish almost instantly" waits: a hang becomes a clear failure.
const Timeout = 5 * time.Second

// FakeClock is a clock that only moves when the test says so.
type FakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func NewFakeClock() *FakeClock {
	return &FakeClock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
}

func (c *FakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *FakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// RecordingSleeper records every requested delay and returns immediately. If it has a FakeClock it advances it
// by each delay, the way real sleeping would. Like the real thing, it refuses to "sleep" on a cancelled context.
type RecordingSleeper struct {
	clock *FakeClock

	mu     sync.Mutex
	delays []time.Duration
}

func NewRecordingSleeper(clock *FakeClock) *RecordingSleeper { return &RecordingSleeper{clock: clock} }

func (s *RecordingSleeper) Sleep(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	s.delays = append(s.delays, d)
	s.mu.Unlock()
	if s.clock != nil {
		s.clock.Advance(d)
	}
	return nil
}

func (s *RecordingSleeper) Delays() []time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]time.Duration(nil), s.delays...)
}
