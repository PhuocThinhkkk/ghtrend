package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// These tests guard the dependency bump performed in go.mod / go.sum
// (Go 1.25.0 toolchain bump plus upgrades of goquery, bubbles, glamour and
// their transitive dependencies). They parse the real go.mod / go.sum files
// at the repository root to make sure the declared versions are the ones
// expected after the bump, that stale pre-bump versions were removed, and
// that go.sum stays consistent with go.mod (every requirement has matching
// checksum entries).

// requireLineRE matches a single dependency line inside a `require ( ... )`
// block, e.g.:
//
//	github.com/PuerkitoBio/goquery v1.12.0
//	github.com/charmbracelet/x/ansi v0.11.6 // indirect
var requireLineRE = regexp.MustCompile(`^\s*(\S+)\s+(v\S+)\s*(//\s*indirect)?\s*$`)

var goVersionRE = regexp.MustCompile(`(?m)^go\s+(\S+)\s*$`)

var moduleLineRE = regexp.MustCompile(`(?m)^module\s+(\S+)\s*$`)

// readGoMod reads go.mod from the repository root. Tests in this file run
// with the working directory set to the package directory (repository
// root), which is where go.mod / go.sum live.
func readGoMod(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatalf("failed to read go.mod: %v", err)
	}
	return string(data)
}

func readGoSum(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("go.sum")
	if err != nil {
		t.Fatalf("failed to read go.sum: %v", err)
	}
	return string(data)
}

// parseRequirements extracts module -> version pairs from every
// `require ( ... )` block found in the given go.mod content.
func parseRequirements(t *testing.T, goModContent string) map[string]string {
	t.Helper()
	reqs := make(map[string]string)
	inBlock := false
	for _, line := range strings.Split(goModContent, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "require (":
			inBlock = true
			continue
		case inBlock && trimmed == ")":
			inBlock = false
			continue
		case inBlock:
			m := requireLineRE.FindStringSubmatch(line)
			if m == nil {
				t.Fatalf("unparsable require line inside require() block: %q", line)
			}
			reqs[m[1]] = m[2]
		}
	}
	return reqs
}

// sumEntry represents the two kinds of hash lines go.sum can have for a
// given module@version: the module content hash ("h1:...") and the go.mod
// file hash ("<version>/go.mod h1:...").
type sumEntry struct {
	hasContentHash bool
	hasGoModHash   bool
}

// parseGoSum builds a map keyed by "module@version" describing which hash
// lines are present in go.sum.
func parseGoSum(t *testing.T, goSumContent string) map[string]sumEntry {
	t.Helper()
	entries := make(map[string]sumEntry)
	for _, line := range strings.Split(goSumContent, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 {
			t.Fatalf("malformed go.sum line (expected 3 fields): %q", line)
		}
		module, versionField, hash := fields[0], fields[1], fields[2]
		if !strings.HasPrefix(hash, "h1:") {
			t.Fatalf("go.sum line has unexpected hash format: %q", line)
		}

		isGoModHash := strings.HasSuffix(versionField, "/go.mod")
		version := strings.TrimSuffix(versionField, "/go.mod")
		key := module + "@" + version

		e := entries[key]
		if isGoModHash {
			e.hasGoModHash = true
		} else {
			e.hasContentHash = true
		}
		entries[key] = e
	}
	return entries
}

func TestGoModDeclaresBumpedGoVersion(t *testing.T) {
	content := readGoMod(t)
	m := goVersionRE.FindStringSubmatch(content)
	if m == nil {
		t.Fatalf("could not find a `go <version>` directive in go.mod:\n%s", content)
	}
	got := m[1]
	want := "1.25.0"
	if got != want {
		t.Errorf("go.mod go directive = %q, want %q", got, want)
	}
}

func TestGoModModulePathUnchanged(t *testing.T) {
	content := readGoMod(t)
	m := moduleLineRE.FindStringSubmatch(content)
	if m == nil {
		t.Fatalf("could not find a `module <path>` directive in go.mod:\n%s", content)
	}
	want := "github.com/PhuocThinhkkk/ghtrend"
	if m[1] != want {
		t.Errorf("module path = %q, want %q", m[1], want)
	}
}

func TestGoModDirectDependencyVersions(t *testing.T) {
	reqs := parseRequirements(t, readGoMod(t))

	// Direct dependencies expected after the bump in this PR.
	wantDirect := map[string]string{
		"github.com/JohannesKaufmann/html-to-markdown/v2": "v2.5.0",
		"github.com/PuerkitoBio/goquery":                  "v1.12.0",
		"github.com/charmbracelet/bubbles":                "v1.0.0",
		"github.com/charmbracelet/bubbletea":              "v1.3.10",
		"github.com/charmbracelet/glamour":                "v1.0.0",
		"github.com/charmbracelet/lipgloss":               "v1.1.1-0.20250404203927-76690c660834",
		"github.com/forPelevin/gomoji":                    "v1.4.1",
		"github.com/lpernett/godotenv":                    "v0.0.0-20230527005122-0de1d4c5ef5e",
		"github.com/spf13/cobra":                          "v1.10.2",
	}

	for module, wantVersion := range wantDirect {
		gotVersion, ok := reqs[module]
		if !ok {
			t.Errorf("go.mod missing expected direct dependency %q", module)
			continue
		}
		if gotVersion != wantVersion {
			t.Errorf("go.mod dependency %q = %s, want %s", module, gotVersion, wantVersion)
		}
	}
}

func TestGoModIndirectDependencyVersions(t *testing.T) {
	reqs := parseRequirements(t, readGoMod(t))

	// A representative sample of indirect dependencies bumped or added by
	// this PR.
	wantIndirect := map[string]string{
		"github.com/alecthomas/chroma/v2":       "v2.20.0",
		"github.com/charmbracelet/colorprofile": "v0.4.1",
		"github.com/charmbracelet/x/ansi":       "v0.11.6",
		"github.com/charmbracelet/x/cellbuf":    "v0.0.15",
		"github.com/charmbracelet/x/term":       "v0.2.2",
		"github.com/clipperhouse/displaywidth":  "v0.9.0",
		"github.com/clipperhouse/stringish":     "v0.1.1",
		"github.com/clipperhouse/uax29/v2":      "v2.5.0",
		"github.com/dlclark/regexp2":            "v1.11.5",
		"github.com/mattn/go-runewidth":         "v0.0.19",
		"github.com/yuin/goldmark-emoji":        "v1.0.6",
		"golang.org/x/net":                      "v0.52.0",
		"golang.org/x/sys":                      "v0.42.0",
		"golang.org/x/term":                     "v0.41.0",
		"golang.org/x/text":                     "v0.35.0",
	}

	for module, wantVersion := range wantIndirect {
		gotVersion, ok := reqs[module]
		if !ok {
			t.Errorf("go.mod missing expected indirect dependency %q", module)
			continue
		}
		if gotVersion != wantVersion {
			t.Errorf("go.mod dependency %q = %s, want %s", module, gotVersion, wantVersion)
		}
	}
}

func TestGoModNoStalePreBumpVersions(t *testing.T) {
	content := readGoMod(t)

	// These module@version pairs existed before the bump in this PR and
	// must not linger in go.mod afterwards.
	stale := []string{
		"github.com/PuerkitoBio/goquery v1.11.0",
		"github.com/charmbracelet/bubbles v0.21.0",
		"github.com/charmbracelet/glamour v0.10.0",
		"github.com/alecthomas/chroma/v2 v2.14.0",
		"github.com/charmbracelet/colorprofile v0.2.3-0.20250311203215-f60798e515dc",
		"github.com/charmbracelet/x/ansi v0.10.1",
		"github.com/charmbracelet/x/cellbuf v0.0.13",
		"github.com/charmbracelet/x/term v0.2.1",
		"github.com/dlclark/regexp2 v1.11.0",
		"github.com/mattn/go-runewidth v0.0.16",
		"github.com/yuin/goldmark-emoji v1.0.5",
		"golang.org/x/net v0.47.0",
		"golang.org/x/sys v0.38.0",
		"golang.org/x/term v0.37.0",
		"golang.org/x/text v0.31.0",
	}

	for _, s := range stale {
		if strings.Contains(content, s) {
			t.Errorf("go.mod still contains stale pre-bump dependency line: %q", s)
		}
	}

	if strings.Contains(content, "go 1.24.2") {
		t.Errorf("go.mod still declares the pre-bump go version 1.24.2")
	}
}

// TestGoSumHasEntriesForEveryGoModRequirement is a regression test for the
// most common failure mode of a manual go.mod edit: bumping a version in
// go.mod without regenerating go.sum, which leaves the checksum database
// out of sync and breaks `go build` / `go test` for anyone without network
// access to refetch sums.
func TestGoSumHasEntriesForEveryGoModRequirement(t *testing.T) {
	reqs := parseRequirements(t, readGoMod(t))
	sums := parseGoSum(t, readGoSum(t))

	for module, version := range reqs {
		key := module + "@" + version
		entry, ok := sums[key]
		if !ok {
			t.Errorf("go.sum has no entries at all for required module %s@%s", module, version)
			continue
		}
		if !entry.hasGoModHash {
			t.Errorf("go.sum missing %q go.mod hash line", key)
		}
		if !entry.hasContentHash {
			t.Errorf("go.sum missing %q module content hash line", key)
		}
	}
}

// TestGoSumNewDependenciesPresent checks that the brand new transitive
// dependencies pulled in by the glamour v1.0.0 upgrade (the clipperhouse
// modules) were correctly recorded with both hash lines in go.sum.
func TestGoSumNewDependenciesPresent(t *testing.T) {
	sums := parseGoSum(t, readGoSum(t))

	newDeps := map[string]string{
		"github.com/clipperhouse/displaywidth": "v0.9.0",
		"github.com/clipperhouse/stringish":    "v0.1.1",
		"github.com/clipperhouse/uax29/v2":     "v2.5.0",
	}

	for module, version := range newDeps {
		key := module + "@" + version
		entry, ok := sums[key]
		if !ok {
			t.Errorf("go.sum missing newly introduced dependency %s@%s", module, version)
			continue
		}
		if !entry.hasGoModHash || !entry.hasContentHash {
			t.Errorf("go.sum entry for %s incomplete: %+v", key, entry)
		}
	}
}

// TestGoSumNoConflictingHashesForSameModuleVersion is a sanity/negative
// check: for any given module@version pair go.sum must not contain two
// different hash values for the same hash kind (content vs go.mod), which
// would indicate a corrupted or hand-edited go.sum.
func TestGoSumNoConflictingHashesForSameModuleVersion(t *testing.T) {
	content := readGoSum(t)

	type key struct {
		module, version string
	}
	contentHashes := make(map[key]string)
	goModHashes := make(map[key]string)

	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 {
			t.Fatalf("malformed go.sum line (expected 3 fields): %q", line)
		}
		module, versionField, hash := fields[0], fields[1], fields[2]

		if strings.HasSuffix(versionField, "/go.mod") {
			k := key{module, strings.TrimSuffix(versionField, "/go.mod")}
			if prev, ok := goModHashes[k]; ok && prev != hash {
				t.Errorf("conflicting go.mod hashes for %s %s: %q vs %q", module, k.version, prev, hash)
			}
			goModHashes[k] = hash
		} else {
			k := key{module, versionField}
			if prev, ok := contentHashes[k]; ok && prev != hash {
				t.Errorf("conflicting content hashes for %s %s: %q vs %q", module, k.version, prev, hash)
			}
			contentHashes[k] = hash
		}
	}
}

// TestGoModRequireBlocksAreWellFormed is a basic structural/negative test
// ensuring go.mod still has the expected two require() blocks (direct and
// indirect) and that every line inside them parses as "<module> <version>"
// optionally followed by "// indirect".
func TestGoModRequireBlocksAreWellFormed(t *testing.T) {
	content := readGoMod(t)

	openCount := strings.Count(content, "require (")
	closeParenAfterRequire := 0
	inBlock := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "require (" {
			inBlock = true
			continue
		}
		if inBlock && trimmed == ")" {
			closeParenAfterRequire++
			inBlock = false
			continue
		}
		if inBlock && trimmed != "" {
			if !requireLineRE.MatchString(line) {
				t.Errorf("malformed line inside require() block: %q", line)
			}
		}
	}

	if openCount != 2 {
		t.Errorf("expected exactly 2 require() blocks in go.mod, found %d", openCount)
	}
	if closeParenAfterRequire != openCount {
		t.Errorf("unbalanced require() blocks: %d opened, %d closed", openCount, closeParenAfterRequire)
	}
}