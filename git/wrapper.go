package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// defaultExcludes are files/patterns we NEVER encrypt
// - Compiled binaries (no extension, executable bit set)
// - Config and lock files that need to stay readable
// - Common non-source files
var defaultExcludes = map[string]bool{
	"sentinel":      true, // the compiled binary itself
	"sentinel.exe":  true,
	"sentinel.toml": true,
	"go.sum":        true, // generated — no point encrypting
	".gitignore":    true,
	"LICENSE":       true,
	"Makefile":      true,
}

// run executes a git command and returns its combined output
func run(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Include stderr in the error message so user sees what git said
		return "", fmt.Errorf("%w\n%s", err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}

// IsGitRepo returns true if the current directory is inside a git repo
func IsGitRepo() (bool, error) {
	_, err := run("rev-parse", "--git-dir")
	if err != nil {
		// Exit code 128 means not a git repo — not a real error
		return false, nil
	}
	return true, nil
}

// AddAll stages all changes (git add .)
func AddAll() error {
	_, err := run("add", ".")
	return err
}

// AddSentinelFiles explicitly stages .sentinel/hashes/ and .sentinel/proofs/
// These directories are created AFTER the first git add . runs, so they
// won't be staged unless we explicitly add them here.
// Keys are excluded — they are in .gitignore and must never be committed.
func AddSentinelFiles() error {
	// Stage hash records and proof files explicitly
	// git add with specific paths — won't touch keys/ because it's in .gitignore
	_, err := run("add",
		".sentinel/hashes/",
		".sentinel/proofs/",
		".sentinel/collaborators.json",
	)
	return err
}

// HasStagedChanges returns true if there are staged changes ready to commit
func HasStagedChanges() (bool, error) {
	out, err := run("diff", "--cached", "--name-only")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// Init runs git init in the current directory
func Init() error {
	_, err := run("init")
	return err
}

// GetTrackedFiles returns all files currently staged (git add) or tracked
func GetTrackedFiles() ([]string, error) {
	// Get staged files
	staged, err := run("diff", "--cached", "--name-only")
	if err != nil {
		return nil, err
	}

	// Get all tracked files (already committed)
	tracked, err := run("ls-files")
	if err != nil {
		return nil, err
	}

	// Combine and deduplicate
	seen := make(map[string]bool)
	var files []string

	for _, f := range append(splitLines(staged), splitLines(tracked)...) {
		f = strings.TrimSpace(f)
		if f == "" || seen[f] {
			continue
		}

		// Skip .sentinel/ internal directory
		if strings.HasPrefix(f, ".sentinel/") {
			continue
		}

		// Skip hardcoded excludes (binaries, config, lock files)
		if defaultExcludes[filepath.Base(f)] {
			continue
		}

		// Skip files with no extension that are executable (compiled binaries)
		if isBinaryFile(f) {
			continue
		}

		// Skip markdown files — keep READMEs human-readable on GitHub
		if strings.HasSuffix(f, ".md") || strings.HasSuffix(f, ".txt") {
			continue
		}

		seen[f] = true
		files = append(files, f)
	}

	return files, nil
}

// isBinaryFile returns true if the file has no extension AND is executable
// This catches compiled Go binaries, C binaries, etc.
func isBinaryFile(path string) bool {
	// If it has a source code extension, it's definitely not a binary
	ext := filepath.Ext(path)
	sourceExts := map[string]bool{
		".go": true, ".js": true, ".ts": true, ".py": true,
		".rs": true, ".c": true, ".cpp": true, ".h": true,
		".java": true, ".rb": true, ".php": true, ".swift": true,
		".kt": true, ".cs": true, ".json": true, ".yaml": true,
		".yml": true, ".toml": true, ".xml": true, ".html": true,
		".css": true, ".sh": true, ".env": true,
	}
	if sourceExts[ext] {
		return false
	}

	// No extension — check if it's executable
	if ext == "" {
		info, err := os.Stat(path)
		if err != nil {
			return false
		}
		// Check executable bit (0111)
		if info.Mode()&0111 != 0 {
			return true // executable with no extension = compiled binary
		}
	}

	return false
}

// Commit runs git commit with the given message and returns the commit hash
func Commit(message string) (string, error) {
	out, err := run("commit", "-m", message)
	if err != nil {
		return "", err
	}

	// Extract the short commit hash from git output
	// git output looks like: [main abc1234] your message
	hash := extractHash(out)
	return hash, nil
}

// Push runs git push
func Push() error {
	_, err := run("push")
	return err
}

// Pull runs git pull
func Pull() error {
	_, err := run("pull")
	return err
}

// Status returns git status output
func Status() (string, error) {
	return run("status")
}

// Log returns recent git log
func Log() (string, error) {
	return run("log", "--oneline", "--graph", "--decorate", "-20")
}

// GetRepoRoot returns the absolute path to the root of the git repo
func GetRepoRoot() (string, error) {
	return run("rev-parse", "--show-toplevel")
}

// GetCurrentBranch returns the current branch name
func GetCurrentBranch() (string, error) {
	return run("rev-parse", "--abbrev-ref", "HEAD")
}

// GetRemoteURL returns the remote origin URL
func GetRemoteURL() (string, error) {
	return run("remote", "get-url", "origin")
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// extractHash pulls the short hash out of git commit output
// e.g. "[main a1b2c3d] my commit" → "a1b2c3d"
func extractHash(output string) string {
	// Look for pattern like "abc1234]"
	for _, word := range strings.Fields(output) {
		word = strings.Trim(word, "[]")
		if len(word) >= 7 && isHex(word) {
			return word
		}
	}
	return "unknown"
}

func isHex(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
