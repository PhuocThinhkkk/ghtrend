package cache

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/PhuocThinhkkk/ghtrend/pkg/configs/flags"
)

const (
	maxHistoryEntries   = 50
	maxHistoryFileBytes = 8 << 20 // 8 MiB
)

// HistoryType identifies the target kind recorded in browsing history.
type HistoryType string

const (
	HistoryTypeUser  HistoryType = "USER"
	HistoryTypeRepo  HistoryType = "REPO"
	HistoryTypeTrend HistoryType = "TREND"
)

// HistoryEntry represents a single visited user, repository, or trending feed.
type HistoryEntry struct {
	ID          string          `json:"id"`
	Type        HistoryType     `json:"type"`
	Target      string          `json:"target"`
	Label       string          `json:"label"`
	Subtitle    string          `json:"subtitle,omitempty"`
	Frequency   flags.Frequency `json:"frequency,omitempty"`
	Language    string          `json:"language,omitempty"`
	LastVisited time.Time       `json:"last_visited"`
	VisitCount  int             `json:"visit_count"`
}

// DefaultCacheDir returns the ghtrend cache directory, honoring GHTREND_CACHE_DIR
// when set and falling back to os.UserCacheDir or ~/.cache.
func DefaultCacheDir() (string, error) {
	if base := strings.TrimSpace(os.Getenv("GHTREND_CACHE_DIR")); base != "" {
		return filepath.Join(base, "ghtrend"), nil
	}
	base, err := os.UserCacheDir()
	if err != nil {
		homeDir, hErr := os.UserHomeDir()
		if hErr != nil {
			return "", err
		}
		base = filepath.Join(homeDir, ".cache")
	}
	return filepath.Join(base, "ghtrend"), nil
}

// DefaultHistoryPath resolves the persistent history file path, honoring
// GHTREND_CACHE_DIR when set and falling back to os.UserCacheDir or ~/.cache.
func DefaultHistoryPath() (string, error) {
	dir, err := DefaultCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "history.json"), nil
}

var (
	errHistoryOversized = errors.New("history file exceeded max allowed size")
	errHistoryCorrupt   = errors.New("history file is corrupt")
)

func isRecoverableHistoryErr(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, errHistoryCorrupt) || errors.Is(err, errHistoryOversized)
}

// HistoryStore provides thread-safe, bounded persistent storage for HistoryEntry items.
type HistoryStore struct {
	mu         sync.Mutex
	path       string
	maxEntries int
	generation uint64
}

// CurrentGen returns the store's current clear generation counter.
func (s *HistoryStore) CurrentGen() uint64 {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.generation
}

// NewHistoryStore constructs a HistoryStore backed by path, capped at 50 entries.
func NewHistoryStore(path string) *HistoryStore {
	return &HistoryStore{
		path:       path,
		maxEntries: maxHistoryEntries,
		generation: 1,
	}
}

// Path returns the backing file path of the store.
func (s *HistoryStore) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

// Load reads and returns all stored history entries sorted newest LastVisited first.
// Missing or empty files return an empty slice and nil error.
func (s *HistoryStore) Load() ([]HistoryEntry, error) {
	if s == nil || strings.TrimSpace(s.path) == "" {
		return nil, errors.New("history store path is empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	return s.loadLocked()
}

// Record inserts or updates a history entry, deduplicating by canonical ID,
// incrementing VisitCount, sorting newest first, capping at 50 entries, and persisting atomically.
func (s *HistoryStore) Record(entry HistoryEntry) error {
	return s.RecordIfGen(entry, 0)
}

// RecordIfGen records entry only if expectedGen is 0 or matches the store's current clear generation.
func (s *HistoryStore) RecordIfGen(entry HistoryEntry, expectedGen uint64) error {
	if s == nil || strings.TrimSpace(s.path) == "" {
		return errors.New("history store path is empty")
	}

	id, err := canonicalHistoryID(entry)
	if err != nil {
		return err
	}
	entry.ID = id
	if entry.LastVisited.IsZero() {
		entry.LastVisited = time.Now().UTC()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if expectedGen != 0 && s.generation != expectedGen {
		return nil
	}

	entries, err := s.loadLocked()
	if err != nil {
		if isRecoverableHistoryErr(err) {
			if qErr := s.quarantineCorruptLocked(); qErr != nil {
				return qErr
			}
			entries = []HistoryEntry{}
		} else {
			return err
		}
	}

	matched := false
	for i := range entries {
		if entries[i].ID == entry.ID {
			entries[i] = mergeHistoryEntry(entries[i], entry)
			matched = true
			break
		}
	}
	if !matched {
		if entry.VisitCount <= 0 {
			entry.VisitCount = 1
		}
		entries = append(entries, entry)
	}

	sortHistoryEntries(entries)
	if s.maxEntries > 0 && len(entries) > s.maxEntries {
		entries = entries[:s.maxEntries]
	}

	return s.writeLocked(entries)
}

// Delete removes any entry matching id and persists the updated history.
// Missing IDs are ignored without error.
func (s *HistoryStore) Delete(id string) error {
	if s == nil || strings.TrimSpace(s.path) == "" {
		return errors.New("history store path is empty")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := s.loadLocked()
	if err != nil {
		return err
	}

	filtered := make([]HistoryEntry, 0, len(entries))
	removed := false
	for _, entry := range entries {
		if entry.ID == id {
			removed = true
			continue
		}
		filtered = append(filtered, entry)
	}
	if !removed {
		return nil
	}

	return s.writeLocked(filtered)
}

// Clear replaces the history file atomically with an empty JSON array.
func (s *HistoryStore) Clear() error {
	if s == nil || strings.TrimSpace(s.path) == "" {
		return errors.New("history store path is empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.generation++
	return s.writeLocked([]HistoryEntry{})
}

func (s *HistoryStore) quarantineCorruptLocked() error {
	corruptPath := s.path + ".corrupt"
	if err := os.Rename(s.path, corruptPath); err == nil {
		return nil
	}
	if removeErr := os.Remove(corruptPath); removeErr == nil {
		if err := os.Rename(s.path, corruptPath); err == nil {
			return nil
		}
	}
	// Fallback to a unique quarantine name if history.json.corrupt cannot be replaced.
	tmpCorrupt, err := os.CreateTemp(filepath.Dir(s.path), filepath.Base(s.path)+".corrupt.*")
	if err != nil {
		return fmt.Errorf("quarantine corrupt history %q: %w", s.path, err)
	}
	altPath := tmpCorrupt.Name()
	_ = tmpCorrupt.Close()
	_ = os.Remove(altPath)
	if err := os.Rename(s.path, altPath); err != nil {
		return fmt.Errorf("quarantine corrupt history %q: %w", s.path, err)
	}
	return nil
}

func (s *HistoryStore) loadLocked() ([]HistoryEntry, error) {
	f, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []HistoryEntry{}, nil
		}
		return nil, fmt.Errorf("open history %q: %w", s.path, err)
	}
	defer f.Close()

	raw, err := io.ReadAll(io.LimitReader(f, maxHistoryFileBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read history %q: %w", s.path, err)
	}
	if len(raw) > maxHistoryFileBytes {
		return nil, fmt.Errorf("%w: %s (%d bytes)", errHistoryOversized, s.path, maxHistoryFileBytes)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return []HistoryEntry{}, nil
	}

	var entries []HistoryEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("%w: decode history %q: %w", errHistoryCorrupt, s.path, err)
	}
	if entries == nil {
		return []HistoryEntry{}, nil
	}

	clean := make([]HistoryEntry, 0, len(entries))
	for _, entry := range entries {
		entry.ID = strings.TrimSpace(entry.ID)
		if entry.ID == "" {
			cid, cidErr := canonicalHistoryID(entry)
			if cidErr != nil {
				continue
			}
			entry.ID = cid
		}
		clean = append(clean, entry)
	}

	sortHistoryEntries(clean)
	if s.maxEntries > 0 && len(clean) > s.maxEntries {
		clean = clean[:s.maxEntries]
	}
	return clean, nil
}

func (s *HistoryStore) writeLocked(entries []HistoryEntry) error {
	if entries == nil {
		entries = []HistoryEntry{}
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create history dir %q: %w", dir, err)
	}

	encoded, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("encode history: %w", err)
	}
	encoded = append(encoded, '\n')
	if len(encoded) > maxHistoryFileBytes {
		return fmt.Errorf("history data exceeded max allowed size of %d bytes", maxHistoryFileBytes)
	}

	return writeFileAtomic(s.path, encoded, "history")
}

func writeFileAtomic(path string, encoded []byte, kind string) error {
	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp %s file: %w", kind, err)
	}
	tmpPath := tmpFile.Name()

	_, writeErr := tmpFile.Write(encoded)
	closeErr := tmpFile.Close()
	if writeErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write temp %s file: %w", kind, writeErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp %s file: %w", kind, closeErr)
	}

	if info, statErr := os.Stat(path); statErr == nil && !info.Mode().IsRegular() {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("replace %s file %q: destination is not a regular file", kind, path)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		// Windows os.Rename may fail if destination exists; fall back to .bak swap.
		bakPath := path + ".bak"
		_ = os.Remove(bakPath)
		if renameBakErr := os.Rename(path, bakPath); renameBakErr == nil {
			if renameErr := os.Rename(tmpPath, path); renameErr != nil {
				_ = os.Rename(bakPath, path)
				_ = os.Remove(tmpPath)
				return fmt.Errorf("replace %s file %q: %w", kind, path, renameErr)
			}
			_ = os.Remove(bakPath)
			return nil
		}
		_ = os.Remove(tmpPath)
		return fmt.Errorf("replace %s file %q: %w", kind, path, err)
	}

	return nil
}

func canonicalHistoryID(entry HistoryEntry) (string, error) {
	if id := strings.TrimSpace(entry.ID); id != "" {
		return id, nil
	}

	switch entry.Type {
	case HistoryTypeUser:
		if username, ok := ParseUserTarget(entry.Target); ok {
			return "user:" + strings.ToLower(username), nil
		}
		target := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(entry.Target), "@"))
		if target != "" {
			return "user:" + strings.ToLower(target), nil
		}
		return "", errors.New("history user entry requires target or id")
	case HistoryTypeRepo:
		if owner, repo, ok := ParseRepoSlug(entry.Target); ok {
			return "repo:" + strings.ToLower(owner+"/"+repo), nil
		}
		target := strings.TrimSpace(entry.Target)
		if target != "" {
			return "repo:" + strings.ToLower(target), nil
		}
		return "", errors.New("history repo entry requires target or id")
	case HistoryTypeTrend:
		since := strings.ToLower(strings.TrimSpace(string(entry.Frequency)))
		lang := strings.ToLower(strings.TrimSpace(entry.Language))
		if (since == "" || lang == "") && strings.TrimSpace(entry.Target) != "" {
			parts := strings.SplitN(strings.TrimSpace(entry.Target), "/", 2)
			if since == "" && strings.TrimSpace(parts[0]) != "" {
				since = strings.ToLower(strings.TrimSpace(parts[0]))
			}
			if lang == "" && len(parts) == 2 {
				lang = strings.ToLower(strings.TrimSpace(parts[1]))
			}
		}
		if since == "" {
			since = string(flags.Daily)
		}
		if lang == "" {
			lang = "all"
		}
		return "trend:" + since + ":" + lang, nil
	default:
		target := strings.TrimSpace(entry.Target)
		if target != "" && strings.TrimSpace(string(entry.Type)) != "" {
			return strings.ToLower(strings.TrimSpace(string(entry.Type))) + ":" + strings.ToLower(target), nil
		}
		return "", errors.New("history entry requires id or target")
	}
}

func mergeHistoryEntry(existing, incoming HistoryEntry) HistoryEntry {
	merged := existing
	if incoming.Type != "" {
		merged.Type = incoming.Type
	}
	if incoming.Target != "" {
		merged.Target = incoming.Target
	}
	if incoming.Label != "" {
		merged.Label = incoming.Label
	}
	if incoming.Subtitle != "" {
		merged.Subtitle = incoming.Subtitle
	}
	if incoming.Frequency != "" {
		merged.Frequency = incoming.Frequency
	}
	if incoming.Language != "" {
		merged.Language = incoming.Language
	}
	merged.LastVisited = incoming.LastVisited
	if merged.VisitCount < 0 {
		merged.VisitCount = 0
	}
	merged.VisitCount++
	return merged
}

func sortHistoryEntries(entries []HistoryEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].LastVisited.After(entries[j].LastVisited)
	})
}

// ParseRepoSlug parses a GitHub repository reference in "owner/repo",
// "github.com/owner/repo", or "https://github.com/owner/repo(.git)?(/...)?" form,
// validating against path traversal and invalid characters.
func ParseRepoSlug(raw string) (owner, repo string, ok bool) {
	s := strings.TrimSpace(raw)
	if s == "" || strings.Contains(s, "\\") || strings.HasPrefix(s, "/") {
		return "", "", false
	}

	var parts []string
	switch {
	case strings.Contains(s, "://"):
		u, err := url.Parse(s)
		if err != nil || u.User != nil {
			return "", "", false
		}
		scheme := strings.ToLower(u.Scheme)
		if scheme != "https" && scheme != "http" {
			return "", "", false
		}
		host := strings.ToLower(u.Host)
		if host != "github.com" && host != "www.github.com" {
			return "", "", false
		}
		trimmedPath := strings.Trim(u.Path, "/")
		if trimmedPath == "" {
			return "", "", false
		}
		parts = strings.Split(trimmedPath, "/")
		if len(parts) < 2 {
			return "", "", false
		}
	case hasGitHubHostPrefix(s):
		u, err := url.Parse("https://" + s)
		if err != nil || u.User != nil {
			return "", "", false
		}
		host := strings.ToLower(u.Host)
		if host != "github.com" && host != "www.github.com" {
			return "", "", false
		}
		trimmedPath := strings.Trim(u.Path, "/")
		if trimmedPath == "" {
			return "", "", false
		}
		parts = strings.Split(trimmedPath, "/")
		if len(parts) < 2 {
			return "", "", false
		}
	default:
		if strings.ContainsAny(s, "?#@:") {
			return "", "", false
		}
		parts = strings.Split(s, "/")
		if len(parts) != 2 {
			return "", "", false
		}
	}

	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", "", false
		}
	}

	owner = parts[0]
	repo = strings.TrimSuffix(parts[1], ".git")
	if strings.HasSuffix(strings.ToLower(repo), ".git") {
		return "", "", false
	}

	if owner == "." || owner == ".." || repo == "." || repo == ".." {
		return "", "", false
	}
	if strings.Contains(owner, "..") || strings.Contains(repo, "..") {
		return "", "", false
	}
	if len(owner) < 1 || len(owner) > 39 || len(repo) < 1 || len(repo) > 100 {
		return "", "", false
	}
	if !isValidRepoSlugPart(owner) || !isValidRepoSlugPart(repo) {
		return "", "", false
	}

	return owner, repo, true
}

// ParseUserTarget parses a GitHub username from "<username>", "@<username>",
// or "https://github.com/<username>", rejecting path traversal and multi-segment paths.
func ParseUserTarget(raw string) (username string, ok bool) {
	s := strings.TrimSpace(raw)
	if s == "" || strings.Contains(s, "\\") {
		return "", false
	}

	if hasGitHubHostPrefix(s) {
		s = "https://" + s
	}

	if strings.Contains(s, "://") {
		u, err := url.Parse(s)
		if err != nil || u.User != nil {
			return "", false
		}
		scheme := strings.ToLower(u.Scheme)
		if scheme != "https" && scheme != "http" {
			return "", false
		}
		host := strings.ToLower(u.Host)
		if host != "github.com" && host != "www.github.com" {
			return "", false
		}
		trimmedPath := strings.Trim(u.Path, "/")
		if trimmedPath == "" || strings.Contains(trimmedPath, "/") {
			return "", false
		}
		s = trimmedPath
	} else {
		s = strings.TrimPrefix(s, "@")
	}

	if len(s) < 1 || len(s) > 39 {
		return "", false
	}
	if s[0] == '-' || s[len(s)-1] == '-' {
		return "", false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
			return "", false
		}
	}

	return s, true
}

func hasGitHubHostPrefix(s string) bool {
	lower := strings.ToLower(s)
	return strings.HasPrefix(lower, "github.com/") || strings.HasPrefix(lower, "www.github.com/")
}

func isValidRepoSlugPart(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		isAlphaNum := (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
		if !isAlphaNum && c != '.' && c != '_' && c != '-' {
			return false
		}
	}
	return true
}
