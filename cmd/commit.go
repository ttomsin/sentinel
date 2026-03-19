package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/sentinel-cli/sentinel/blockchain"
	"github.com/sentinel-cli/sentinel/crypto"
	"github.com/sentinel-cli/sentinel/git"
	"github.com/spf13/cobra"
)

var commitCmd = &cobra.Command{
	Use:   "commit [flags] -m <message>",
	Short: "Hash, encrypt, and commit your code",
	Long: `Hash your code, encrypt it, then commit via Git.

DEFAULT MODE — full protection:
  1. SHA-256 hash every source file     → proof of authorship
  2. AES-256-GCM encrypt every file     → AI sees only noise on GitHub
  3. git commit (encrypted blobs)
  4. Decrypt locally — you keep working normally
  5. Anchor root hash to Bitcoin        → free, permanent, immutable proof

PROOF-ONLY MODE (--proof-only):
  Use this for open source projects that want authorship proof
  WITHOUT encrypting the code. Perfect for Sentinel itself.

  1. SHA-256 hash every source file     → proof of authorship
  2. git commit (plaintext — readable by everyone)
  3. Anchor root hash to Bitcoin        → free, permanent, immutable proof

  No encryption. Code stays readable. Authorship is still proven on Bitcoin.

Examples:
  sentinel commit -m "add feature"              # full protection
  sentinel commit -m "release v1" --proof-only  # open source proof`,

	RunE: func(cmd *cobra.Command, args []string) error {
		msg, _ := cmd.Flags().GetString("message")
		if msg == "" {
			return fmt.Errorf("commit message required: use -m \"your message\"")
		}
		proofOnly, _ := cmd.Flags().GetBool("proof-only")
		return runCommit(msg, proofOnly)
	},
}

func init() {
	commitCmd.Flags().StringP("message", "m", "", "Commit message (required)")
	commitCmd.Flags().Bool("proof-only", false, "Hash and anchor to Bitcoin without encrypting — for open source projects")
}

func runCommit(message string, proofOnly bool) error {
	bold := color.New(color.Bold)
	green := color.New(color.FgGreen, color.Bold)
	yellow := color.New(color.FgYellow)
	red := color.New(color.FgRed, color.Bold)
	cyan := color.New(color.FgCyan)

	fmt.Println()
	if proofOnly {
		bold.Println("  Sentinel Commit — Proof Only Mode")
		cyan.Println("  (no encryption — code stays readable)")
	} else {
		bold.Println("  Sentinel Commit")
	}
	fmt.Println()

	// ── Key check — only needed for encryption mode ───────────────────────────
	var aesKey []byte
	if !proofOnly {
		yellow.Print("  → Checking encryption keys... ")
		if !crypto.KeysExist() {
			red.Println("NOT FOUND")
			return fmt.Errorf("no keys found — run 'sentinel keygen' first\n  (or use --proof-only for open source projects)")
		}
		green.Println("found.")

		yellow.Print("  → Loading AES-256 key... ")
		var err error
		aesKey, err = crypto.LoadAESKey()
		if err != nil {
			red.Println("FAILED")
			return fmt.Errorf("failed to load key: %w", err)
		}
		green.Println("done.")
	} else {
		cyan.Println("  → Skipping encryption keys (proof-only mode)")
	}

	// ── Stage all changes ─────────────────────────────────────────────────────
	yellow.Print("  → Staging all changes (git add .)... ")
	if err := git.AddAll(); err != nil {
		red.Println("FAILED")
		return fmt.Errorf("git add failed: %w", err)
	}
	green.Println("done.")

	// ── Check there's something to commit ────────────────────────────────────
	yellow.Print("  → Checking for changes to commit... ")
	hasChanges, err := git.HasStagedChanges()
	if err != nil {
		red.Println("FAILED")
		return fmt.Errorf("failed to check git status: %w", err)
	}
	if !hasChanges {
		yellow.Println("\n\n  Nothing to commit — no changes detected since last commit.")
		fmt.Println()
		return nil
	}
	green.Println("changes found.")

	// ── Collect source files ──────────────────────────────────────────────────
	yellow.Print("  → Collecting tracked files... ")
	files, err := git.GetTrackedFiles()
	if err != nil {
		red.Println("FAILED")
		return fmt.Errorf("failed to get tracked files: %w", err)
	}
	green.Printf("%d files found.\n", len(files))

	// ── SHA-256 hash every file — PROVE layer (always runs) ──────────────────
	yellow.Print("  → Hashing plaintext files (SHA-256)... ")
	hashes, err := crypto.HashFiles(files)
	if err != nil {
		red.Println("FAILED")
		return fmt.Errorf("hashing failed: %w", err)
	}

	timestamp := time.Now().UTC()
	hashFile, rootHash, err := crypto.SaveHashes(hashes, timestamp)
	if err != nil {
		red.Println("FAILED")
		return fmt.Errorf("failed to save hashes: %w", err)
	}
	green.Println("done.")
	cyan.Printf("     Root hash:  %s...\n", rootHash[:32])
	cyan.Printf("     Hash file:  %s\n", hashFile)

	// ── Register proof record synchronously ───────────────────────────────────
	_, err = blockchain.RegisterProof(rootHash, hashFile)
	if err != nil {
		yellow.Printf("  ⚠  Warning: could not register proof record: %v\n", err)
	}

	// ── Encrypt files — PREVENT layer (skipped in proof-only mode) ───────────
	if !proofOnly {
		yellow.Print("  → Encrypting files (AES-256-GCM)... ")
		if err := crypto.EncryptFiles(files, aesKey); err != nil {
			red.Println("FAILED")
			return fmt.Errorf("encryption failed: %w", err)
		}
		green.Println("done.")

		// CRITICAL: re-stage encrypted versions so Git commits ciphertext not plaintext
		yellow.Print("  → Staging encrypted files... ")
		if err := git.AddAll(); err != nil {
			_ = crypto.DecryptFiles(files, aesKey)
			red.Println("FAILED")
			return fmt.Errorf("failed to stage encrypted files: %w", err)
		}
		green.Println("done.")
	} else {
		cyan.Println("  → Skipping encryption (proof-only mode)")
	}

	// ── Git commit ────────────────────────────────────────────────────────────
	yellow.Print("  → Running git commit... ")
	commitHash, err := git.Commit(message)
	if err != nil {
		// If encryption mode — decrypt back before failing
		if !proofOnly && aesKey != nil {
			_ = crypto.DecryptFiles(files, aesKey)
		}
		red.Println("FAILED")

		errStr := err.Error()
		if strings.Contains(errStr, "nothing to commit") {
			return fmt.Errorf("nothing to commit — no changes detected")
		}
		if strings.Contains(errStr, "Please tell me who you are") {
			return fmt.Errorf("git user not configured — run:\n  git config --global user.email \"you@example.com\"\n  git config --global user.name \"Your Name\"")
		}
		return fmt.Errorf("git commit failed: %w", err)
	}
	green.Println("done.")

	// ── Decrypt back locally — only in encryption mode ───────────────────────
	if !proofOnly {
		yellow.Print("  → Restoring plaintext locally... ")
		if err := crypto.DecryptFiles(files, aesKey); err != nil {
			red.Println("FAILED")
			return fmt.Errorf(
				"CRITICAL: files left encrypted — run 'sentinel pull' to restore: %w", err,
			)
		}
		green.Println("done.")
	}

	// ── Anchor to Bitcoin via OpenTimestamps — always runs ────────────────────
	yellow.Println("  → Anchoring to Bitcoin (OpenTimestamps, async)...")
	go func(rh, hf string) {
		_, _ = blockchain.AnchorHash(rh, hf)
	}(rootHash, hashFile)

	// ── Summary ───────────────────────────────────────────────────────────────
	fmt.Println()
	green.Println("  ✓ Commit successful!")
	fmt.Println()
	fmt.Printf("  Git commit:  %s\n", commitHash)
	fmt.Printf("  Message:     %s\n", message)
	fmt.Printf("  Files:       %d\n", len(files))
	fmt.Printf("  Root hash:   %s...\n", rootHash[:32])
	fmt.Printf("  Timestamp:   %s\n", timestamp.Format(time.RFC3339))

	if proofOnly {
		cyan.Printf("  Mode:        proof-only (no encryption — code is readable)\n")
	} else {
		green.Printf("  Mode:        full protection (encrypted on GitHub)\n")
	}

	yellow.Printf("  Blockchain:  anchoring to Bitcoin in background...\n")
	fmt.Println()
	fmt.Println("  Run 'sentinel push'         to push to remote.")
	fmt.Println("  Run 'sentinel proof status' to check Bitcoin confirmation.")
	fmt.Println()

	return nil
}
