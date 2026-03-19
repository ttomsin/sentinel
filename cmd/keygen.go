package cmd

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/sentinel-cli/sentinel/crypto"
	"github.com/spf13/cobra"
)

var keygenCmd = &cobra.Command{
	Use:   "keygen",
	Short: "Generate your master encryption key pair",
	Long: `Generate a master key pair for this repository.

This creates two files in .sentinel/keys/:
  master.key   — Your PRIVATE key. Never share this. Never commit this.
  master.pub   — Your PUBLIC key. Safe to share with collaborators.

The private key is used to:
  - Decrypt your code locally after pulling
  - Sign proof certificates
  - Derive keys for collaborators (sentinel grant)

Keys use Ed25519 for signing and AES-256 for encryption.`,

	RunE: func(cmd *cobra.Command, args []string) error {
		return runKeygen()
	},
}

func runKeygen() error {
	bold := color.New(color.Bold)
	green := color.New(color.FgGreen, color.Bold)
	yellow := color.New(color.FgYellow)
	red := color.New(color.FgRed, color.Bold)

	fmt.Println()
	bold.Println("  Generating Master Key Pair...")
	fmt.Println()

	// Check if keys already exist
	if crypto.KeysExist() {
		yellow.Println("  ⚠  Keys already exist in .sentinel/keys/")
		fmt.Println("     To regenerate, delete .sentinel/keys/ and run again.")
		fmt.Println("     WARNING: This will lock you out of existing encrypted commits.")
		fmt.Println()
		return nil
	}

	// Generate AES-256 symmetric encryption key (for encrypting code)
	yellow.Print("  → Generating AES-256 encryption key... ")
	aesKey, err := crypto.GenerateAESKey()
	if err != nil {
		red.Println("FAILED")
		return fmt.Errorf("AES key generation failed: %w", err)
	}
	green.Println("done.")

	// Generate Ed25519 key pair (for signing + identity)
	yellow.Print("  → Generating Ed25519 signing key pair... ")
	privateKey, publicKey, err := crypto.GenerateKeyPair()
	if err != nil {
		red.Println("FAILED")
		return fmt.Errorf("key pair generation failed: %w", err)
	}
	green.Println("done.")

	// Save keys to disk
	yellow.Print("  → Saving keys to .sentinel/keys/... ")
	if err := crypto.SaveKeys(aesKey, privateKey, publicKey); err != nil {
		red.Println("FAILED")
		return fmt.Errorf("failed to save keys: %w", err)
	}
	green.Println("done.")

	// Print summary
	fmt.Println()
	green.Println("  ✓ Keys generated successfully!")
	fmt.Println()

	pubKeyHex, _ := crypto.PublicKeyFingerprint(publicKey)
	fmt.Printf("  Public Key Fingerprint:\n")
	color.New(color.FgCyan, color.Bold).Printf("  %s\n", pubKeyHex)
	fmt.Println()

	red.Println("  ⚠  IMPORTANT:")
	fmt.Println("     .sentinel/keys/master.key is your PRIVATE key.")
	fmt.Println("     It is already in .gitignore — but back it up securely.")
	fmt.Println("     Losing it means losing access to all encrypted commits.")
	fmt.Println()

	return nil
}
