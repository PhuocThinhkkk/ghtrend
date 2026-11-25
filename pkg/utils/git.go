package utils

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type gitRunner func(ctx context.Context, args []string) error

// ValidateCloneRepo validates that owner and repo are safe GitHub slug segments
// suitable for constructing a clone URL and local target directory name.
func ValidateCloneRepo(owner, repo string) error {
	if err := validateCloneSlug("owner", owner); err != nil {
		return err
	}
	if err := validateCloneSlug("repo", repo); err != nil {
		return err
	}
	return nil
}

// RepoCloneURL validates owner and repo and returns the HTTPS GitHub clone URL.
func RepoCloneURL(owner, repo string) (string, error) {
	if err := ValidateCloneRepo(owner, repo); err != nil {
		return "", err
	}
	return fmt.Sprintf("https://github.com/%s/%s.git", owner, repo), nil
}

// CloneRepository clones https://github.com/{owner}/{repo}.git into filepath.Join(parentDir, repo).
// If parentDir is empty or whitespace, the current working directory "." is used.
func CloneRepository(ctx context.Context, owner, repo, parentDir string) (string, error) {
	return cloneRepositoryWithRunner(ctx, owner, repo, parentDir, defaultGitRunner)
}

func cloneRepositoryWithRunner(
	ctx context.Context,
	owner, repo, parentDir string,
	runner gitRunner,
) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	targetPath, args, err := prepareClone(owner, repo, parentDir)
	if err != nil {
		return "", err
	}

	if err := runner(ctx, args); err != nil {
		return "", err
	}

	return targetPath, nil
}

func prepareClone(owner, repo, parentDir string) (string, []string, error) {
	cloneURL, err := RepoCloneURL(owner, repo)
	if err != nil {
		return "", nil, err
	}

	resolvedParent := strings.TrimSpace(parentDir)
	if resolvedParent == "" {
		resolvedParent = "."
	} else {
		resolvedParent = filepath.Clean(resolvedParent)
	}

	targetPath := filepath.Join(resolvedParent, repo)
	if _, err := os.Stat(targetPath); err == nil {
		return "", nil, fmt.Errorf("target directory already exists: %s", targetPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", nil, fmt.Errorf("checking target directory %s: %w", targetPath, err)
	}

	args := []string{"clone", "--", cloneURL, targetPath}
	return targetPath, args, nil
}

func defaultGitRunner(ctx context.Context, args []string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return fmt.Errorf("git %s failed: %w: %s", args[0], err, msg)
		}
		return fmt.Errorf("git %s failed: %w", args[0], err)
	}
	return nil
}

func validateCloneSlug(field, value string) error {
	if value == "" {
		return fmt.Errorf("%s must not be empty", field)
	}
	if value == "." || value == ".." || strings.Contains(value, "..") {
		return fmt.Errorf("invalid %s: %q", field, value)
	}
	if strings.HasPrefix(value, "-") {
		return fmt.Errorf("invalid %s: must not start with '-'", field)
	}
	for i := 0; i < len(value); i++ {
		if !isSafeSlugChar(value[i]) {
			return fmt.Errorf("invalid %s: contains disallowed character %q", field, value[i])
		}
	}
	return nil
}

func isSafeSlugChar(c byte) bool {
	return (c >= 'A' && c <= 'Z') ||
		(c >= 'a' && c <= 'z') ||
		(c >= '0' && c <= '9') ||
		c == '.' || c == '_' || c == '-'
}
