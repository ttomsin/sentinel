package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/sentinel-cli/sentinel/config"
	"github.com/sentinel-cli/sentinel/git"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Sentinel in a Git repository",
	Long: `Initialize Sentinel protection in the current Git repository.

This will:
  - Verify a Git repository exists (or create one)
  - Create a .sentinel/ directory for config and keys
  - Add .sentinel/keys/ to .gitignore (your keys never leave your machine)
  - Create a sentinel.toml config file`,

	RunE: func(cmd *cobra.Command, args []string) error {
		return runInit()
	},
}

func runInit() error {
	bold := color.New(color.Bold)
	green := color.New(color.FgGreen, color.Bold)
	yellow := color.New(color.FgYellow)
	red := color.New(color.FgRed, color.Bold)

	fmt.Println()
	bold.Println("  Initializing Sentinel...")
	fmt.Println()

	// Step 1: Check we are inside a git repo
	yellow.Print("  → Checking for Git repository... ")
	isRepo, err := git.IsGitRepo()
	if err != nil {
		red.Println("ERROR")
		return fmt.Errorf("failed to check git repo: %w", err)
	}

	if !isRepo {
		// No git repo — offer to create one
		yellow.Println("none found.")
		yellow.Print("  → Running git init... ")
		if err := git.Init(); err != nil {
			red.Println("FAILED")
			return fmt.Errorf("git init failed: %w", err)
		}
		green.Println("done.")
	} else {
		green.Println("found.")
	}

	// Step 2: Create .sentinel directory structure
	yellow.Print("  → Creating .sentinel/ directory... ")
	dirs := []string{
		".sentinel",
		".sentinel/keys",
		".sentinel/hashes",
		".sentinel/proofs",
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0700); err != nil {
			red.Println("FAILED")
			return fmt.Errorf("failed to create %s: %w", dir, err)
		}
	}
	green.Println("done.")

	// Step 3: Add .sentinel/keys to .gitignore (CRITICAL — keys must never be pushed)
	yellow.Print("  → Protecting keys in .gitignore... ")
	if err := ensureGitignore(); err != nil {
		red.Println("FAILED")
		return fmt.Errorf("failed to update .gitignore: %w", err)
	}
	green.Println("done.")

	// Step 4: Create sentinel.toml config
	yellow.Print("  → Writing sentinel.toml config... ")
	if err := config.WriteDefault(); err != nil {
		red.Println("FAILED")
		return fmt.Errorf("failed to write config: %w", err)
	}
	green.Println("done.")

	// Step 5: Done!
	fmt.Println()
	green.Println("  ✓ Sentinel initialized successfully!")
	fmt.Println()
	fmt.Println("  Next steps:")
	color.New(color.FgCyan).Println("    sentinel keygen     # Generate your master encryption key pair")
	color.New(color.FgCyan).Println("    sentinel commit     # Make your first protected commit")
	fmt.Println()

	return nil
}

// ensureGitignore adds sentinel key paths to .gitignore
func ensureGitignore() error {
	entries := []string{
		"\n# Sentinel — keys must NEVER be committed",
		".sentinel/keys/",
		".sentinel.key",
	}

	// Read existing .gitignore
	existing, err := os.ReadFile(".gitignore")
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	content := string(existing)

	// Append entries if not already present
	for _, entry := range entries {
		if !containsLine(content, entry) {
			content += "\n" + entry
		}
	}

	return os.WriteFile(filepath.Join(".", ".gitignore"), []byte(content), 0644)
}

// containsLine checks if a string contains a specific line
func containsLine(content, line string) bool {
	for _, l := range splitLines(content) {
		if l == line {
			return true
		}
	}
	return false
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i, c := range s {
		if c == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
