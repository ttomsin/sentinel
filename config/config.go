package config

import (
	"fmt"
	"os"
	"time"
)

const ConfigFile = "sentinel.toml"

// WriteDefault creates a default sentinel.toml in the current directory
func WriteDefault() error {
	// Check if config already exists
	if _, err := os.Stat(ConfigFile); err == nil {
		return nil // Already exists, don't overwrite
	}

	content := fmt.Sprintf(`# Sentinel Configuration
# Generated: %s
# Docs: https://sentinel-cli.dev/docs

[sentinel]
version = "1.0"

[encryption]
# Algorithm used to encrypt your code before pushing
algorithm = "AES-256-GCM"

# File patterns to exclude from encryption (e.g. README, LICENSE)
exclude = [
  "README.md",
  "LICENSE",
  "*.md",
  ".gitignore",
  ".sentinel/**",
]

[hashing]
# Algorithm used to fingerprint your code
algorithm = "SHA-256"

# Automatically hash on every sentinel commit
auto_hash = true

[blockchain]
# Blockchain network for anchoring hashes
# Options: "ethereum", "bitcoin", "polygon", "disabled"
network = "disabled"   # Enable in Phase 3

# Anchor every commit or batch them
anchor_mode = "async"  # "sync", "async", "batch"

[detect]
# Phase 4 — Similarity detection
enabled = false

[collaborators]
# Phase 5 — Access control
# Managed via 'sentinel grant' and 'sentinel revoke'
`, time.Now().Format(time.RFC3339))

	return os.WriteFile(ConfigFile, []byte(content), 0644)
}