package cache

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSessionCache_DefaultCapacity(t *testing.T) {
	sc := NewSessionCache(0, time.Minute)
	if sc.maxEntries != defaultSessionMaxEntries {
		t.Fatalf("expected default maxEntries=%d, got %d", defaultSessionMaxEntries, sc.maxEntries)
	}

	for i := 0; i < defaultSessionMaxEntries+20; i++ {
		sc.SetBytes(fmt.Sprintf("k-%d", i), []byte("v"), 0)
	}
	if got := sc.Len(); got != defaultSessionMaxEntries {
		t.Fatalf("expected Len=%d after overflow, got %d", defaultSessionMaxEntries, got)
	}
}

func TestSessionCache_BytesRoundTripAndDefensiveCopy(t *testing.T) {
	sc := NewSessionCache(8, time.Minute)

	input := []byte("hello-world")
	sc.SetBytes("k1", input, 0)
	input[0] = 'X' // mutate caller input

	got, ok := sc.GetBytes("k1")
	if !ok {
		t.Fatalf("expected hit for k1")
	}
	if !bytes.Equal(got, []byte("hello-world")) {
		t.Fatalf("expected unmodified bytes 'hello-world', got %q", got)
	}

	got[0] = 'Y' // mutate returned slice
	got2, ok := sc.GetBytes("k1")
	if !ok {
		t.Fatalf("expected second hit for k1")
	}
	if !bytes.Equal(got2, []byte("hello-world")) {
		t.Fatalf("cache was mutated by caller slice modification: got %q", got2)
	}

	// Empty non-nil and nil slices
	sc.SetBytes("empty", []byte{}, 0)
	emptyGot, ok := sc.GetBytes("empty")
	if !ok || emptyGot == nil || len(emptyGot) != 0 {
		t.Fatalf("expected non-nil empty slice, got (%#v, %v)", emptyGot, ok)
	}

	sc.SetBytes("nil", nil, 0)
	nilGot, ok := sc.GetBytes("nil")
	if !ok || nilGot != nil {
		t.Fatalf("expected nil slice with ok=true, got (%#v, %v)", nilGot, ok)
	}

	if _, ok := sc.GetBytes("missing"); ok {
		t.Fatalf("expected miss for unknown key")
	}
}

func TestSessionCache_TTLExpiry(t *testing.T) {
	tests := []struct {
		name       string
		defaultTTL time.Duration
		itemTTL    time.Duration
		wait       time.Duration
		wantHit    bool
	}{
		{
			name:       "uses default TTL when item TTL <= 0",
			defaultTTL: 25 * time.Millisecond,
			itemTTL:    0,
			wait:       45 * time.Millisecond,
			wantHit:    false,
		},
		{
			name:       "explicit item TTL overrides default TTL",
			defaultTTL: time.Minute,
			itemTTL:    20 * time.Millisecond,
			wait:       40 * time.Millisecond,
			wantHit:    false,
		},
		{
			name:       "unexpired entry remains cached",
			defaultTTL: time.Minute,
			itemTTL:    0,
			wait:       5 * time.Millisecond,
			wantHit:    true,
		},
		{
			name:       "non-positive default and item TTL never expire",
			defaultTTL: 0,
			itemTTL:    0,
			wait:       20 * time.Millisecond,
			wantHit:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sc := NewSessionCache(4, tc.defaultTTL)
			sc.SetBytes("k", []byte("val"), tc.itemTTL)
			time.Sleep(tc.wait)

			_, ok := sc.GetBytes("k")
			if ok != tc.wantHit {
				t.Fatalf("GetBytes ok=%v, want %v", ok, tc.wantHit)
			}
			wantLen := 0
			if tc.wantHit {
				wantLen = 1
			}
			if gotLen := sc.Len(); gotLen != wantLen {
				t.Fatalf("Len=%d, want %d", gotLen, wantLen)
			}
		})
	}
}

func TestSessionCache_LRUEviction(t *testing.T) {
	sc := NewSessionCache(3, time.Minute)
	sc.SetBytes("a", []byte("1"), 0)
	sc.SetBytes("b", []byte("2"), 0)
	sc.SetBytes("c", []byte("3"), 0)

	// Access "a" so "b" becomes least recently used.
	if _, ok := sc.GetBytes("a"); !ok {
		t.Fatalf("expected hit for a")
	}

	// Inserting "d" should evict "b".
	sc.SetBytes("d", []byte("4"), 0)

	if _, ok := sc.GetBytes("b"); ok {
		t.Fatalf("expected 'b' to be evicted")
	}
	for _, k := range []string{"a", "c", "d"} {
		if _, ok := sc.GetBytes(k); !ok {
			t.Fatalf("expected %q to remain in cache", k)
		}
	}

	// Updating existing key should not increase Len or evict others.
	sc.SetBytes("a", []byte("1b"), 0)
	if sc.Len() != 3 {
		t.Fatalf("expected Len=3 after updating existing key, got %d", sc.Len())
	}
}

func TestSessionCache_ValuesAndInvalidation(t *testing.T) {
	type sample struct {
		ID int
	}

	sc := NewSessionCache(10, time.Minute)
	sc.SetValue("repo:1", sample{ID: 1}, 0)
	sc.SetValue("repo:2", sample{ID: 2}, 0)
	sc.SetValue("user:alice", sample{ID: 3}, 0)

	// Value entries do not leak through GetBytes.
	if _, ok := sc.GetBytes("repo:1"); ok {
		t.Fatalf("expected GetBytes miss for SetValue entry")
	}

	v, ok := sc.GetValue("repo:1")
	if !ok || v.(sample).ID != 1 {
		t.Fatalf("expected sample{ID:1}, got (%v, %v)", v, ok)
	}

	sc.Invalidate("repo:1")
	if _, ok := sc.GetValue("repo:1"); ok {
		t.Fatalf("expected repo:1 to be invalidated")
	}

	sc.InvalidatePrefix("repo:")
	if _, ok := sc.GetValue("repo:2"); ok {
		t.Fatalf("expected repo:2 to be invalidated by prefix")
	}
	if _, ok := sc.GetValue("user:alice"); !ok {
		t.Fatalf("expected user:alice to remain after repo: prefix invalidation")
	}

	// Empty prefix clears everything.
	sc.InvalidatePrefix("")
	if sc.Len() != 0 {
		t.Fatalf("expected Len=0 after empty prefix invalidation, got %d", sc.Len())
	}

	sc.SetBytes("x", []byte("1"), 0)
	sc.Clear()
	if sc.Len() != 0 {
		t.Fatalf("expected Len=0 after Clear, got %d", sc.Len())
	}
}

func TestSessionCache_GetOrFetchBytes_DeduplicationAndNonBlocking(t *testing.T) {
	sc := NewSessionCache(16, time.Minute)

	var calls atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})

	const workers = 12
	var wg sync.WaitGroup
	wg.Add(workers)

	results := make([][]byte, workers)
	fromCache := make([]bool, workers)
	errs := make([]error, workers)

	for i := 0; i < workers; i++ {
		idx := i
		go func() {
			defer wg.Done()
			data, hit, err := sc.GetOrFetchBytes(context.Background(), "trending:go:daily", 0, func(ctx context.Context) ([]byte, error) {
				if calls.Add(1) == 1 {
					close(started)
				}
				<-release
				return []byte("payload"), nil
			})
			results[idx] = data
			fromCache[idx] = hit
			errs[idx] = err
		}()
	}

	<-started
	waitForInflightWaiters(t, sc, "trending:go:daily", workers)

	// Cache mutex must NOT be held across fn I/O: unrelated keys must be accessible immediately.
	doneOther := make(chan struct{})
	go func() {
		sc.SetBytes("other", []byte("fast"), 0)
		_, _ = sc.GetBytes("other")
		close(doneOther)
	}()

	select {
	case <-doneOther:
	case <-time.After(time.Second):
		t.Fatalf("cache mutex appears blocked while GetOrFetchBytes fn is running")
	}

	close(release)
	wg.Wait()

	if calls.Load() != 1 {
		t.Fatalf("expected fn to run exactly once, ran %d times", calls.Load())
	}
	for i := 0; i < workers; i++ {
		if errs[i] != nil {
			t.Fatalf("worker %d unexpected err: %v", i, errs[i])
		}
		if fromCache[i] {
			t.Fatalf("worker %d expected fromCache=false for singleflight fetch", i)
		}
		if !bytes.Equal(results[i], []byte("payload")) {
			t.Fatalf("worker %d got %q, want 'payload'", i, results[i])
		}
	}

	// Mutating one caller's result must not affect another or the cache.
	results[0][0] = 'Z'
	if !bytes.Equal(results[1], []byte("payload")) {
		t.Fatalf("coalesced callers shared backing slice")
	}

	// Subsequent call must hit cache without calling fn again.
	cached, hit, err := sc.GetOrFetchBytes(context.Background(), "trending:go:daily", 0, func(ctx context.Context) ([]byte, error) {
		calls.Add(1)
		return []byte("should-not-run"), nil
	})
	if err != nil || !hit || !bytes.Equal(cached, []byte("payload")) {
		t.Fatalf("expected cache hit with 'payload', got (%q, %v, %v)", cached, hit, err)
	}
	if calls.Load() != 1 {
		t.Fatalf("expected no additional fn call on cache hit, got %d", calls.Load())
	}
}

func waitForInflightWaiters(t *testing.T, sc *SessionCache, key string, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		sc.mu.Lock()
		call := sc.inflight[key]
		waiters := 0
		if call != nil {
			waiters = call.waiters
		}
		sc.mu.Unlock()
		if waiters >= want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d inflight waiters on key %q", want, key)
}

func TestSessionCache_GetOrFetchBytes_ErrorsPanicsAndContext(t *testing.T) {
	sc := NewSessionCache(8, time.Minute)

	// 1. Fetch errors are not cached.
	wantErr := errors.New("boom")
	_, hit, err := sc.GetOrFetchBytes(context.Background(), "err-key", 0, func(ctx context.Context) ([]byte, error) {
		return nil, wantErr
	})
	if !errors.Is(err, wantErr) || hit {
		t.Fatalf("expected (%v, false), got (%v, %v)", wantErr, err, hit)
	}
	if _, ok := sc.GetBytes("err-key"); ok {
		t.Fatalf("errored fetch should not be cached")
	}

	// 2. Panic inside fn is recovered as error.
	_, _, err = sc.GetOrFetchBytes(context.Background(), "panic-key", 0, func(ctx context.Context) ([]byte, error) {
		panic("unexpected crash")
	})
	if err == nil {
		t.Fatalf("expected error from recovered panic")
	}

	// 3. Nil fn returns error on miss.
	if _, _, err := sc.GetOrFetchBytes(context.Background(), "nil-fn", 0, nil); err == nil {
		t.Fatalf("expected error when fn is nil")
	}

	// 4. Pre-cancelled context does not invoke fn.
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	var ran atomic.Bool
	_, _, err = sc.GetOrFetchBytes(cancelledCtx, "cancelled", 0, func(ctx context.Context) ([]byte, error) {
		ran.Store(true)
		return []byte("x"), nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if ran.Load() {
		t.Fatalf("fn should not run for already cancelled context")
	}

	// 5. One waiter cancels while another waiter remains active: active waiter succeeds.
	ctx1, cancel1 := context.WithCancel(context.Background())
	fnStarted := make(chan struct{})
	fnRelease := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(2)

	var err1, err2 error
	var res2 []byte

	go func() {
		defer wg.Done()
		_, _, err1 = sc.GetOrFetchBytes(ctx1, "shared-cancel", 0, func(ctx context.Context) ([]byte, error) {
			close(fnStarted)
			<-fnRelease
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return []byte("ok"), nil
		})
	}()

	<-fnStarted
	go func() {
		defer wg.Done()
		res2, _, err2 = sc.GetOrFetchBytes(context.Background(), "shared-cancel", 0, func(ctx context.Context) ([]byte, error) {
			return []byte("unexpected"), nil
		})
	}()

	waitForInflightWaiters(t, sc, "shared-cancel", 2)
	cancel1()
	time.Sleep(15 * time.Millisecond)
	close(fnRelease)
	wg.Wait()

	if !errors.Is(err1, context.Canceled) {
		t.Fatalf("expected waiter 1 to receive context.Canceled, got %v", err1)
	}
	if err2 != nil || !bytes.Equal(res2, []byte("ok")) {
		t.Fatalf("expected waiter 2 to succeed with 'ok', got (%q, %v)", res2, err2)
	}

	// 6. Single waiter cancels: fn's ctx is cancelled too.
	ctxSingle, cancelSingle := context.WithCancel(context.Background())
	fnSawCancel := make(chan struct{})
	fnBegun := make(chan struct{})

	var singleErr error
	var wgSingle sync.WaitGroup
	wgSingle.Add(1)
	go func() {
		defer wgSingle.Done()
		_, _, singleErr = sc.GetOrFetchBytes(ctxSingle, "single-cancel", 0, func(ctx context.Context) ([]byte, error) {
			close(fnBegun)
			<-ctx.Done()
			close(fnSawCancel)
			return nil, ctx.Err()
		})
	}()

	<-fnBegun
	cancelSingle()
	wgSingle.Wait()

	select {
	case <-fnSawCancel:
	case <-time.After(time.Second):
		t.Fatalf("expected fn context to be cancelled when sole waiter cancelled")
	}
	if !errors.Is(singleErr, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", singleErr)
	}
}

func TestSessionCache_InvalidateDuringInflightPreventsStaleStore(t *testing.T) {
	sc := NewSessionCache(8, time.Minute)
	started := make(chan struct{})
	release := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _, _ = sc.GetOrFetchBytes(context.Background(), "k", 0, func(ctx context.Context) ([]byte, error) {
			close(started)
			<-release
			return []byte("stale"), nil
		})
	}()

	<-started
	sc.Invalidate("k")
	close(release)
	wg.Wait()

	if _, ok := sc.GetBytes("k"); ok {
		t.Fatalf("expected invalidated in-flight result not to be stored in cache")
	}
}

func TestSessionCache_ExplicitWriteDuringInflightIsNotOverwritten(t *testing.T) {
	sc := NewSessionCache(8, time.Minute)
	started := make(chan struct{})
	release := make(chan struct{})

	var wg sync.WaitGroup
	var fetchRes []byte
	var fetchErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		fetchRes, _, fetchErr = sc.GetOrFetchBytes(context.Background(), "k-write", 0, func(ctx context.Context) ([]byte, error) {
			close(started)
			<-release
			return []byte("older-fetch"), nil
		})
	}()

	<-started
	sc.SetBytes("k-write", []byte("newer-explicit"), time.Minute)
	close(release)
	wg.Wait()

	if fetchErr != nil || !bytes.Equal(fetchRes, []byte("older-fetch")) {
		t.Fatalf("expected waiter to receive fetch result, got (%q, %v)", fetchRes, fetchErr)
	}
	got, ok := sc.GetBytes("k-write")
	if !ok || !bytes.Equal(got, []byte("newer-explicit")) {
		t.Fatalf("expected explicit SetBytes value preserved, got (%q, %v)", got, ok)
	}
}

func TestSessionCache_ConcurrentRace(t *testing.T) {
	sc := NewSessionCache(32, 30*time.Millisecond)
	var wg sync.WaitGroup

	for g := 0; g < 16; g++ {
		id := g
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				key := fmt.Sprintf("prefix-%d:key-%d", id%3, i%20)
				switch i % 6 {
				case 0:
					sc.SetBytes(key, []byte{byte(i)}, 20*time.Millisecond)
				case 1:
					_, _ = sc.GetBytes(key)
				case 2:
					sc.SetValue(key+":val", i, 20*time.Millisecond)
				case 3:
					_, _ = sc.GetValue(key + ":val")
				case 4:
					_, _, _ = sc.GetOrFetchBytes(context.Background(), key, 20*time.Millisecond, func(ctx context.Context) ([]byte, error) {
						return []byte("fetched"), nil
					})
				case 5:
					if i%30 == 5 {
						sc.InvalidatePrefix("prefix-0:")
					} else {
						_ = sc.Len()
					}
				}
			}
		}()
	}
	wg.Wait()
}
