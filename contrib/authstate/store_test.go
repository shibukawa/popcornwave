package authstate

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestMemoryStoreTakeIsSingleUse(t *testing.T) {
	now := time.Unix(100, 0)
	store, err := NewMemoryStore[string](Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(context.Background(), "state", "value", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}

	var successes atomic.Int32
	var wait sync.WaitGroup
	for range 32 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			value, err := store.Take(context.Background(), "state")
			if err == nil {
				if value != "value" {
					t.Errorf("Take value = %q", value)
				}
				successes.Add(1)
				return
			}
			if !errors.Is(err, ErrNotFound) {
				t.Errorf("Take error = %v", err)
			}
		}()
	}
	wait.Wait()
	if got := successes.Load(); got != 1 {
		t.Fatalf("successful Take count = %d, want 1", got)
	}
}

func TestMemoryStoreConcurrentPutAndTake(t *testing.T) {
	store, err := NewMemoryStore[int](Options{})
	if err != nil {
		t.Fatal(err)
	}
	const workers = 64
	var wait sync.WaitGroup
	errorsCh := make(chan error, workers)
	for index := 0; index < workers; index++ {
		index := index
		wait.Add(1)
		go func() {
			defer wait.Done()
			key := "state-" + strconv.Itoa(index)
			if err := store.Put(context.Background(), key, index, time.Now().Add(time.Minute)); err != nil {
				errorsCh <- err
				return
			}
			value, err := store.Take(context.Background(), key)
			if err != nil {
				errorsCh <- err
				return
			}
			if value != index {
				errorsCh <- errors.New("stored value mismatch")
			}
		}()
	}
	wait.Wait()
	close(errorsCh)
	for err := range errorsCh {
		t.Errorf("concurrent Put/Take error = %v", err)
	}
}

func TestMemoryStoreExpiryConsumesValue(t *testing.T) {
	now := time.Unix(100, 0)
	store, err := NewMemoryStore[int](Options{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(context.Background(), "state", 7, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Second)
	if _, err := store.Take(context.Background(), "state"); !errors.Is(err, ErrExpired) {
		t.Fatalf("first Take error = %v, want ErrExpired", err)
	}
	if _, err := store.Take(context.Background(), "state"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second Take error = %v, want ErrNotFound", err)
	}
}

func TestMemoryStoreHonorsCancellation(t *testing.T) {
	store, err := NewMemoryStore[int](Options{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := store.Put(ctx, "state", 1, time.Now().Add(time.Minute)); !errors.Is(err, context.Canceled) {
		t.Fatalf("Put error = %v", err)
	}
	if _, err := store.Take(ctx, "state"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Take error = %v", err)
	}
}

func TestMemoryStoreRejectsNilContext(t *testing.T) {
	store, err := NewMemoryStore[int](Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(nil, "state", 1, time.Now().Add(time.Minute)); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("Put(nil) = %v", err)
	}
	if _, err := store.Take(nil, "state"); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("Take(nil) = %v", err)
	}
}

func TestNilMemoryStoreIsSafe(t *testing.T) {
	var store *MemoryStore[int]
	if err := store.Put(context.Background(), "state", 1, time.Now().Add(time.Minute)); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("nil Put = %v", err)
	}
	if _, err := store.Take(context.Background(), "state"); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("nil Take = %v", err)
	}
}

func TestZeroMemoryStoreIsSafe(t *testing.T) {
	var store MemoryStore[int]
	if err := store.Put(context.Background(), "state", 1, time.Now().Add(time.Minute)); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("zero Put = %v", err)
	}
	if _, err := store.Take(context.Background(), "state"); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("zero Take = %v", err)
	}
}

func TestMemoryStoreLimitsAndDuplicates(t *testing.T) {
	now := time.Unix(100, 0)
	store, err := NewMemoryStore[int](Options{
		Now: func() time.Time { return now }, MaxEntries: 1, MaxKeyBytes: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(context.Background(), "first", 1, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(context.Background(), "first", 2, now.Add(time.Minute)); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("duplicate Put error = %v", err)
	}
	if err := store.Put(context.Background(), "other", 2, now.Add(time.Minute)); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("limited Put error = %v", err)
	}
	if err := store.Put(context.Background(), "longer", 2, now.Add(time.Minute)); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("long-key Put error = %v", err)
	}
}

func TestMemoryStoreReclaimsExpiredCapacity(t *testing.T) {
	now := time.Unix(100, 0)
	store, err := NewMemoryStore[int](Options{Now: func() time.Time { return now }, MaxEntries: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(context.Background(), "expired", 1, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Second)
	if err := store.Put(context.Background(), "fresh", 2, now.Add(time.Minute)); err != nil {
		t.Fatalf("Put after expiry = %v", err)
	}
	if value, err := store.Take(context.Background(), "fresh"); err != nil || value != 2 {
		t.Fatalf("fresh Take = %d, %v", value, err)
	}
}

func TestMemoryStoreRejectsUnboundedConfiguration(t *testing.T) {
	if _, err := NewMemoryStore[int](Options{MaxEntries: hardMaxEntries + 1}); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("oversized MaxEntries error = %v", err)
	}
	if _, err := NewMemoryStore[int](Options{MaxKeyBytes: hardMaxKeyBytes + 1}); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("oversized MaxKeyBytes error = %v", err)
	}
}
