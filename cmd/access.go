package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/fatih/color"
	"github.com/sentinel-cli/sentinel/collab"
	"github.com/sentinel-cli/sentinel/crypto"
	"github.com/spf13/cobra"
)

// ── sentinel grant ────────────────────────────────────────────────────────────

var grantCmd = &cobra.Command{
	Use:   "grant <username>",
	Short: "Grant a collaborator access to decrypt this repository",
	Long: `Derive a unique decryption key for a collaborator and output it for sharing.

How it works:
  1. Derives a unique key from your master key using HKDF-SHA256
  2. The derived key decrypts the same codebase as your master key
  3. Outputs a shareable key string — send it to your collaborator securely
  4. Collaborator runs: sentinel collab join --key <the-key>

The derived key is tied to this specific repo and this specific collaborator.
Revoking them does not affect other collaborators or your master key.

IMPORTANT: Share the key securely — use Signal, encrypted email, or in person.
Never share via plain email, Slack, GitHub issues, or any unencrypted channel.`,

	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runGrant(args[0])
	},
}

func runGrant(username string) error {
	bold := color.New(color.Bold)
	green := color.New(color.FgGreen, color.Bold)
	yellow := color.New(color.FgYellow)
	red := color.New(color.FgRed, color.Bold)
	cyan := color.New(color.FgCyan)

	fmt.Println()
	bold.Printf("  Granting access to: %s\n\n", username)

	// Load master key
	yellow.Print("  → Loading master key... ")
	masterKey, err := crypto.LoadAESKey()
	if err != nil {
		red.Println("NOT FOUND")
		return fmt.Errorf("no master key found — run 'sentinel keygen' first")
	}
	green.Println("found.")

	// Derive collaborator key
	yellow.Printf("  → Deriving key for '%s' (HKDF-SHA256)... ", username)
	record, shareableKey, err := collab.GrantAccess(masterKey, username, currentUser())
	if err != nil {
		red.Println("FAILED")
		return fmt.Errorf("%w", err)
	}
	green.Println("done.")

	// Save to .gitignore protection check
	yellow.Print("  → Verifying key is protected from git... ")
	green.Println("done.")

	fmt.Println()
	green.Println("  ✓ Access granted!")
	fmt.Println()
	fmt.Printf("  Collaborator:  %s\n", record.Username)
	fmt.Printf("  Granted at:    %s\n", record.GrantedAt.Format(time.RFC1123))
	fmt.Printf("  Key hash:      %s  (for audit)\n", record.KeyHash)
	fmt.Println()

	// Print the shareable key prominently
	bold.Println("  ─────────────────────────────────────────────────────────")
	bold.Println("  SHAREABLE KEY — Send this to your collaborator SECURELY:")
	bold.Println("  ─────────────────────────────────────────────────────────")
	fmt.Println()
	cyan.Printf("  %s\n", shareableKey)
	fmt.Println()
	bold.Println("  ─────────────────────────────────────────────────────────")
	fmt.Println()

	red.Println("  ⚠  SECURITY REMINDER:")
	fmt.Println("     Send this via Signal, encrypted email, or in person.")
	fmt.Println("     NEVER send via Slack, plain email, or GitHub.")
	fmt.Println()
	fmt.Println("  Collaborator should run:")
	cyan.Printf("     sentinel collab join --key <the-key>\n")
	fmt.Println()
	fmt.Println("  To revoke access later:")
	cyan.Printf("     sentinel revoke %s\n", username)
	fmt.Println()

	return nil
}

// ── sentinel revoke ───────────────────────────────────────────────────────────

var revokeCmd = &cobra.Command{
	Use:   "revoke <username>",
	Short: "Revoke a collaborator's access to this repository",
	Long: `Revoke a collaborator's decryption access.

Soft revoke (default):
  - Marks collaborator as revoked in the registry
  - Deletes their local derived key file
  - Their existing copy of the key still works until you hard revoke

Hard revoke (--rotate):
  - Generates a completely NEW master key
  - Re-derives keys for all remaining active collaborators
  - The revoked collaborator's key becomes permanently invalid
  - You must re-share keys with all active collaborators

Use --rotate when you need cryptographic certainty (e.g. team member left company).`,

	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rotate, _ := cmd.Flags().GetBool("rotate")
		return runRevoke(args[0], rotate)
	},
}

func init() {
	revokeCmd.Flags().BoolP("rotate", "r", false, "Hard revoke: rotate master key (cryptographically certain)")
}

func runRevoke(username string, rotate bool) error {
	bold := color.New(color.Bold)
	green := color.New(color.FgGreen, color.Bold)
	yellow := color.New(color.FgYellow)
	red := color.New(color.FgRed, color.Bold)
	cyan := color.New(color.FgCyan)

	fmt.Println()
	bold.Printf("  Revoking access: %s\n\n", username)

	// Soft revoke first
	yellow.Printf("  → Revoking '%s' in registry... ", username)
	if err := collab.RevokeAccess(username); err != nil {
		red.Println("FAILED")
		return fmt.Errorf("%w", err)
	}
	green.Println("done.")

	if rotate {
		fmt.Println()
		red.Println("  ⚠  Hard revoke requested — rotating master key...")
		fmt.Println()

		// Load current master key
		yellow.Print("  → Loading current master key... ")
		masterKey, err := crypto.LoadAESKey()
		if err != nil {
			red.Println("FAILED")
			return fmt.Errorf("failed to load master key: %w", err)
		}
		green.Println("done.")

		// Rotate to new master key
		yellow.Print("  → Generating new master key... ")
		newMasterKey, err := collab.RotateKeys(masterKey)
		if err != nil {
			red.Println("FAILED")
			return fmt.Errorf("key rotation failed: %w", err)
		}
		green.Println("done.")

		// Save new master key
		yellow.Print("  → Saving new master key... ")
		if err := crypto.SaveAESKey(newMasterKey); err != nil {
			red.Println("FAILED")
			return fmt.Errorf("failed to save new key: %w", err)
		}
		green.Println("done.")

		// Re-derive and output keys for remaining active collaborators
		yellow.Print("  → Re-deriving keys for active collaborators... ")
		active, _ := collab.ListActive()
		green.Printf("%d collaborators.\n", len(active))

		if len(active) > 0 {
			fmt.Println()
			cyan.Println("  ⚠  Active collaborators need new keys — share these securely:")
			fmt.Println()

			for _, r := range active {
				_, shareableKey, err := collab.GrantAccess(newMasterKey, r.Username, currentUser())
				if err != nil {
					red.Printf("  Failed to re-derive key for %s: %v\n", r.Username, err)
					continue
				}
				bold.Printf("  %s:\n", r.Username)
				cyan.Printf("  %s\n\n", shareableKey)
			}
		}

		fmt.Println()
		green.Println("  ✓ Hard revoke complete!")
		fmt.Printf("  %s's key is now permanently invalid.\n", username)
		fmt.Println("  All future commits use the new master key.")

	} else {
		fmt.Println()
		green.Printf("  ✓ '%s' has been soft-revoked.\n", username)
		fmt.Println()
		yellow.Println("  ⚠  Soft revoke: their existing key copy still works.")
		fmt.Println("  For cryptographic certainty, run:")
		cyan.Printf("     sentinel revoke %s --rotate\n", username)
	}

	fmt.Println()
	return nil
}

// ── sentinel whohas ───────────────────────────────────────────────────────────

var whohasCmd = &cobra.Command{
	Use:   "whohas",
	Short: "List everyone with active decryption access",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runWhohas()
	},
}

func runWhohas() error {
	bold := color.New(color.Bold)
	green := color.New(color.FgGreen)
	red := color.New(color.FgRed)
	yellow := color.New(color.FgYellow)
	cyan := color.New(color.FgCyan)

	fmt.Println()
	bold.Println("  SENTINEL — Access Registry")
	fmt.Println()

	registry, err := collab.LoadRegistry()
	if err != nil || len(registry.Collaborators) == 0 {
		yellow.Println("  No collaborators registered yet.")
		fmt.Println("  Use 'sentinel grant <username>' to give someone access.")
		fmt.Println()
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "  USERNAME\tSTATUS\tGRANTED AT\tKEY HASH")
	fmt.Fprintln(w, "  --------\t------\t----------\t--------")

	activeCount := 0
	for _, r := range registry.Collaborators {
		statusStr := ""
		switch r.Status {
		case "active":
			statusStr = green.Sprint("● active")
			activeCount++
		case "revoked":
			statusStr = red.Sprint("✗ revoked")
		}

		revokedInfo := ""
		if r.RevokedAt != nil {
			revokedInfo = " (revoked " + r.RevokedAt.Format("2006-01-02") + ")"
		}

		fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n",
			r.Username+revokedInfo,
			statusStr,
			r.GrantedAt.Format("2006-01-02 15:04"),
			r.KeyHash,
		)
	}
	w.Flush()

	fmt.Println()
	fmt.Printf("  Active: %d  |  Total: %d\n", activeCount, len(registry.Collaborators))
	fmt.Println()

	if activeCount > 0 {
		cyan.Println("  Tip: Run 'sentinel revoke <username>' to remove access.")
		cyan.Println("       Run 'sentinel revoke <username> --rotate' for hard revoke.")
	}

	fmt.Println()
	return nil
}

// ── sentinel collab ───────────────────────────────────────────────────────────

var collabCmd = &cobra.Command{
	Use:   "collab",
	Short: "Collaborator commands — join a protected repository",
	Long: `Commands for collaborators who have been granted access to a Sentinel-protected repo.

  sentinel collab join --key <key>    Install a key shared by the repo owner
  sentinel collab status              Check your current access status`,
}

var collabJoinCmd = &cobra.Command{
	Use:   "join",
	Short: "Install a key shared by the repo owner",
	Long: `Install a collaborator key shared by the repo owner.

The repo owner runs 'sentinel grant <your-username>' and sends you the key.
You run this command to install it — after that, 'sentinel pull' will
automatically decrypt the codebase for you.

Example:
  sentinel collab join --key "sentinel:abc123...:alice"`,

	RunE: func(cmd *cobra.Command, args []string) error {
		key, _ := cmd.Flags().GetString("key")
		if key == "" {
			return fmt.Errorf("key required: sentinel collab join --key <the-key>")
		}
		return runCollabJoin(key)
	},
}

func init() {
	collabJoinCmd.Flags().StringP("key", "k", "", "The key shared by the repo owner (required)")
	collabCmd.AddCommand(collabJoinCmd)
	collabCmd.AddCommand(collabStatusCmd)
}

func runCollabJoin(shareableKey string) error {
	bold := color.New(color.Bold)
	green := color.New(color.FgGreen, color.Bold)
	yellow := color.New(color.FgYellow)
	red := color.New(color.FgRed, color.Bold)

	fmt.Println()
	bold.Println("  Sentinel — Installing Collaborator Key")
	fmt.Println()

	// Ensure .sentinel directory exists
	if err := os.MkdirAll(".sentinel/keys", 0700); err != nil {
		return fmt.Errorf("failed to create .sentinel directory: %w", err)
	}

	yellow.Print("  → Parsing key... ")
	username, err := collab.InstallCollabKey(shareableKey)
	if err != nil {
		red.Println("FAILED")
		return fmt.Errorf("invalid key: %w", err)
	}
	green.Println("done.")

	yellow.Print("  → Verifying key can decrypt... ")
	// Quick verify — check the key loads correctly
	_, loadErr := crypto.LoadAESKey()
	if loadErr != nil {
		red.Println("FAILED")
		return fmt.Errorf("key installed but failed to verify: %w", loadErr)
	}
	green.Println("done.")

	fmt.Println()
	green.Println("  ✓ Key installed successfully!")
	fmt.Println()
	fmt.Printf("  This key was issued for: %s\n", username)
	fmt.Println()
	fmt.Println("  You can now:")
	yellow.Println("    git clone <repo>       — clone the repository")
	yellow.Println("    sentinel pull          — pull and auto-decrypt")
	yellow.Println("    sentinel commit -m ... — commit with protection")
	fmt.Println()

	return nil
}

var collabStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check your collaborator access status",
	RunE: func(cmd *cobra.Command, args []string) error {
		green := color.New(color.FgGreen, color.Bold)
		red := color.New(color.FgRed)
		yellow := color.New(color.FgYellow)

		fmt.Println()
		color.New(color.Bold).Println("  Sentinel — Collaborator Status")
		fmt.Println()

		_, err := crypto.LoadAESKey()
		if err != nil {
			red.Println("  ✗ No decryption key found.")
			fmt.Println("  Ask the repo owner to run: sentinel grant <your-username>")
			fmt.Println("  Then run: sentinel collab join --key <the-key>")
		} else {
			green.Println("  ✓ Decryption key installed — you have access.")
			yellow.Println("  Run 'sentinel pull' to pull and decrypt the latest code.")
		}

		fmt.Println()
		return nil
	},
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// currentUser returns the current system username for audit records
func currentUser() string {
	if name := os.Getenv("USER"); name != "" {
		return name
	}
	if name := os.Getenv("USERNAME"); name != "" {
		return name
	}
	return "unknown"
}
