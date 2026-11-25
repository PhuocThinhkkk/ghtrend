package cache

import (
	"container/list"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

const defaultSessionMaxEntries = 256

type sessionEntry struct {
	key       string
	bytes     []byte
	hasBytes  bool
	val       any
	hasVal    bool
	expiresAt time.Time
}

type inflightCall struct {
	done    chan struct{}
	waiters int
	cancel  context.CancelFunc
	val     []byte
	err     error
}

// SessionCache is a thread-safe, bounded in-memory cache with TTL expiry,
// LRU eviction, and per-key in-flight fetch deduplication.
type SessionCache struct {
	mu         sync.RWMutex
	maxEntries int
	defaultTTL time.Duration
	entries    map[string]*list.Element
	order      *list.List
	inflight   map[string]*inflightCall
}

// NewSessionCache creates a SessionCache with the given capacity and default TTL.
// If maxEntries <= 0, capacity defaults to 256.
func NewSessionCache(maxEntries int, defaultTTL time.Duration) *SessionCache {
	if maxEntries <= 0 {
		maxEntries = defaultSessionMaxEntries
	}
	return &SessionCache{
		maxEntries: maxEntries,
		defaultTTL: defaultTTL,
		entries:    make(map[string]*list.Element, maxEntries),
		order:      list.New(),
		inflight:   make(map[string]*inflightCall),
	}
}

// GetBytes retrieves a defensive copy of the cached byte slice for key.
func (s *SessionCache) GetBytes(key string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getBytesLocked(key, time.Now())
}

// SetBytes stores a defensive copy of val under key with the given TTL.
// If ttl <= 0, the cache's defaultTTL is used.
func (s *SessionCache) SetBytes(key string, val []byte, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.setEntryLocked(key, sessionEntry{
		key:      key,
		bytes:    cloneBytes(val),
		hasBytes: true,
	}, ttl, time.Now())
}

// GetValue retrieves an arbitrary cached value for key.
func (s *SessionCache) GetValue(key string) (any, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	elem, ok := s.entries[key]
	if !ok {
		return nil, false
	}
	entry := elem.Value.(*sessionEntry)
	if isExpired(entry, time.Now()) {
		s.removeElementLocked(elem)
		return nil, false
	}
	if !entry.hasVal {
		return nil, false
	}
	s.order.MoveToFront(elem)
	return entry.val, true
}

// SetValue stores an arbitrary value under key with the given TTL.
// If ttl <= 0, the cache's defaultTTL is used.
func (s *SessionCache) SetValue(key string, val any, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.setEntryLocked(key, sessionEntry{
		key:    key,
		val:    val,
		hasVal: true,
	}, ttl, time.Now())
}

// Invalidate removes key from the cache and detaches any matching in-flight call.
func (s *SessionCache) Invalidate(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if elem, ok := s.entries[key]; ok {
		s.removeElementLocked(elem)
	}
	delete(s.inflight, key)
}

// InvalidatePrefix removes all entries whose keys start with prefix.
func (s *SessionCache) InvalidatePrefix(prefix string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if prefix == "" {
		s.clearLocked()
		return
	}
	for k, elem := range s.entries {
		if strings.HasPrefix(k, prefix) {
			s.removeElementLocked(elem)
		}
	}
	for k := range s.inflight {
		if strings.HasPrefix(k, prefix) {
			delete(s.inflight, k)
		}
	}
}

// Clear removes all cached entries and detaches all in-flight calls.
func (s *SessionCache) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clearLocked()
}

// Len returns the number of non-expired entries currently stored in the cache.
func (s *SessionCache) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	count := 0
	for _, elem := range s.entries {
		entry := elem.Value.(*sessionEntry)
		if !isExpired(entry, now) {
			count++
		}
	}
	return count
}

// GetOrFetchBytes returns cached bytes for key if present and unexpired (fromCache=true).
// On a miss, concurrent callers for the same key share a single execution of fn without
// holding the cache mutex across I/O. Successful results are cached with ttl.
func (s *SessionCache) GetOrFetchBytes(
	ctx context.Context,
	key string,
	ttl time.Duration,
	fn func(context.Context) ([]byte, error),
) ([]byte, bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	s.mu.Lock()
	if cached, ok := s.getBytesLocked(key, time.Now()); ok {
		s.mu.Unlock()
		return cached, true, nil
	}
	if err := ctx.Err(); err != nil {
		s.mu.Unlock()
		return nil, false, err
	}
	if fn == nil {
		s.mu.Unlock()
		return nil, false, fmt.Errorf("session cache fetch function is nil")
	}

	call, ok := s.inflight[key]
	if ok {
		call.waiters++
		s.mu.Unlock()
	} else {
		callCtx, callCancel := context.WithCancel(context.WithoutCancel(ctx))
		call = &inflightCall{
			done:    make(chan struct{}),
			waiters: 1,
			cancel:  callCancel,
		}
		s.inflight[key] = call
		s.mu.Unlock()

		go s.runFetch(callCtx, key, ttl, call, fn)
	}

	select {
	case <-call.done:
		if call.err != nil {
			return nil, false, call.err
		}
		return cloneBytes(call.val), false, nil
	case <-ctx.Done():
		s.mu.Lock()
		call.waiters--
		if call.waiters == 0 {
			if s.inflight[key] == call {
				delete(s.inflight, key)
			}
			call.cancel()
		}
		s.mu.Unlock()
		return nil, false, ctx.Err()
	}
}

func (s *SessionCache) runFetch(
	callCtx context.Context,
	key string,
	ttl time.Duration,
	call *inflightCall,
	fn func(context.Context) ([]byte, error),
) {
	defer call.cancel()

	var (
		res []byte
		err error
	)
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("session cache fetch panic: %v", r)
			}
		}()
		res, err = fn(callCtx)
	}()

	cloned := cloneBytes(res)

	s.mu.Lock()
	stillActive := s.inflight[key] == call
	if stillActive {
		delete(s.inflight, key)
	}
	if err == nil && callCtx.Err() == nil && stillActive {
		s.setEntryLocked(key, sessionEntry{
			key:      key,
			bytes:    cloneBytes(cloned),
			hasBytes: true,
		}, ttl, time.Now())
	}
	call.val = cloned
	call.err = err
	close(call.done)
	s.mu.Unlock()
}

func (s *SessionCache) getBytesLocked(key string, now time.Time) ([]byte, bool) {
	elem, ok := s.entries[key]
	if !ok {
		return nil, false
	}
	entry := elem.Value.(*sessionEntry)
	if isExpired(entry, now) {
		s.removeElementLocked(elem)
		return nil, false
	}
	if !entry.hasBytes {
		return nil, false
	}
	s.order.MoveToFront(elem)
	return cloneBytes(entry.bytes), true
}

func (s *SessionCache) setEntryLocked(key string, entry sessionEntry, ttl time.Duration, now time.Time) {
	if ttl <= 0 {
		ttl = s.defaultTTL
	}
	if ttl > 0 {
		entry.expiresAt = now.Add(ttl)
	}

	if elem, ok := s.entries[key]; ok {
		elem.Value = &entry
		s.order.MoveToFront(elem)
		return
	}

	s.evictExpiredLocked(now)
	for len(s.entries) >= s.maxEntries {
		oldest := s.order.Back()
		if oldest == nil {
			break
		}
		s.removeElementLocked(oldest)
	}

	elem := s.order.PushFront(&entry)
	s.entries[key] = elem
}

func (s *SessionCache) evictExpiredLocked(now time.Time) {
	for _, elem := range s.entries {
		entry := elem.Value.(*sessionEntry)
		if isExpired(entry, now) {
			s.removeElementLocked(elem)
		}
	}
}

func (s *SessionCache) removeElementLocked(elem *list.Element) {
	entry := elem.Value.(*sessionEntry)
	delete(s.entries, entry.key)
	s.order.Remove(elem)
}

func (s *SessionCache) clearLocked() {
	s.entries = make(map[string]*list.Element, s.maxEntries)
	s.order.Init()
	s.inflight = make(map[string]*inflightCall)
}

func isExpired(entry *sessionEntry, now time.Time) bool {
	return !entry.expiresAt.IsZero() && !now.Before(entry.expiresAt)
}

func cloneBytes(b []byte) []byte {
	if b == nil {
		return nil
	}
	cp := make([]byte, len(b))
	copy(cp, b)
	return cp
}
