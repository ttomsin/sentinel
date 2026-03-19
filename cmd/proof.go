package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/fatih/color"
	"github.com/sentinel-cli/sentinel/blockchain"
	"github.com/spf13/cobra"
)

// proofCmd is now a parent command with subcommands
var proofCmd = &cobra.Command{
	Use:   "proof",
	Short: "Manage blockchain proof certificates",
	Long: `Manage blockchain-anchored proof of authorship certificates.

Subcommands:
  sentinel proof status          Show status of the latest proof
  sentinel proof list            List all proofs for this repo
  sentinel proof upgrade         Try to upgrade pending proofs to confirmed
  sentinel proof verify <hash>   Verify a specific root hash has a proof`,

	// If no subcommand given, show status by default
	RunE: func(cmd *cobra.Command, args []string) error {
		return runProofStatus()
	},
}

func init() {
	// Register proof subcommands
	proofCmd.AddCommand(proofStatusCmd)
	proofCmd.AddCommand(proofListCmd)
	proofCmd.AddCommand(proofUpgradeCmd)
	proofCmd.AddCommand(proofVerifyCmd)
}

// ── sentinel proof status ─────────────────────────────────────────────────────

var proofStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of the latest proof",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runProofStatus()
	},
}

func runProofStatus() error {
	bold := color.New(color.Bold)
	green := color.New(color.FgGreen, color.Bold)
	yellow := color.New(color.FgYellow)
	cyan := color.New(color.FgCyan)
	red := color.New(color.FgRed)

	fmt.Println()
	bold.Println("  SENTINEL PROOF — Latest Status")
	fmt.Println()

	record, err := blockchain.GetLatestProof()
	if err != nil {
		yellow.Println("  No proofs found yet.")
		fmt.Println("  Run 'sentinel commit -m \"message\"' to create your first proof.")
		fmt.Println()
		return nil
	}

	// Display the proof details
	cyan.Println("  Root Hash:")
	fmt.Printf("    %s\n", record.RootHash)
	fmt.Println()

	// Status with colour
	fmt.Print("  Status:      ")
	switch record.Status {
	case "confirmed":
		green.Println("✓ CONFIRMED on Bitcoin")
	case "pending":
		yellow.Println("⏳ PENDING — awaiting Bitcoin confirmation (up to 24hrs)")
	case "failed":
		red.Println("✗ FAILED — try 'sentinel proof upgrade'")
	}

	fmt.Printf("  Submitted:   %s\n", record.SubmittedAt.Format(time.RFC1123))
	fmt.Printf("  .ots file:   %s\n", record.OTSFile)
	fmt.Printf("  Calendar:    %s\n", record.Server)

	if record.Status == "confirmed" && record.BitcoinTx != "" {
		fmt.Println()
		green.Println("  Bitcoin Proof:")
		fmt.Printf("    Transaction: %s\n", record.BitcoinTx)
		fmt.Printf("    Block:       %d\n", record.BitcoinBlock)
		cyan.Printf("    Verify at:   https://blockstream.info/tx/%s\n", record.BitcoinTx)
	}

	fmt.Println()

	// Offer to upgrade if pending
	if record.Status == "pending" {
		yellow.Println("  Tip: Run 'sentinel proof upgrade' to check if Bitcoin has confirmed it.")
	}

	fmt.Println()
	return nil
}

// ── sentinel proof list ───────────────────────────────────────────────────────

var proofListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all proof records for this repository",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runProofList()
	},
}

func runProofList() error {
	bold := color.New(color.Bold)
	green := color.New(color.FgGreen)
	yellow := color.New(color.FgYellow)
	red := color.New(color.FgRed)

	fmt.Println()
	bold.Println("  SENTINEL PROOF — All Records")
	fmt.Println()

	records, err := blockchain.ListProofs()
	if err != nil || len(records) == 0 {
		yellow.Println("  No proof records found.")
		fmt.Println("  Run 'sentinel commit' to generate your first proof.")
		fmt.Println()
		return nil
	}

	// Use a tab writer for aligned columns
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "  #\tHASH\tSTATUS\tSUBMITTED")
	fmt.Fprintln(w, "  -\t----\t------\t---------")

	for i, r := range records {
		statusStr := ""
		switch r.Status {
		case "confirmed":
			statusStr = green.Sprint("✓ confirmed")
		case "pending":
			statusStr = yellow.Sprint("⏳ pending")
		case "failed":
			statusStr = red.Sprint("✗ failed")
		}

		fmt.Fprintf(w, "  %d\t%s\t%s\t%s\n",
			i+1,
			blockchain.ShortHash(r.RootHash),
			statusStr,
			r.SubmittedAt.Format("2006-01-02 15:04:05"),
		)
	}
	w.Flush()

	fmt.Println()
	fmt.Printf("  Total: %d proof(s)\n", len(records))
	fmt.Println()
	return nil
}

// ── sentinel proof upgrade ────────────────────────────────────────────────────

var proofUpgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade pending proofs to confirmed Bitcoin timestamps",
	Long: `Check if pending .ots proof files have been confirmed on Bitcoin.

OpenTimestamps batches hashes and anchors them to Bitcoin every ~1-24 hours.
Run this command periodically to upgrade your PENDING proofs to CONFIRMED.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runProofUpgrade()
	},
}

func runProofUpgrade() error {
	bold := color.New(color.Bold)
	green := color.New(color.FgGreen, color.Bold)
	yellow := color.New(color.FgYellow)

	fmt.Println()
	bold.Println("  SENTINEL PROOF — Upgrading Pending Proofs")
	fmt.Println()

	records, err := blockchain.ListProofs()
	if err != nil || len(records) == 0 {
		yellow.Println("  No proof records found.")
		fmt.Println()
		return nil
	}

	pending := 0
	upgraded := 0

	for _, record := range records {
		if record.Status != "pending" {
			continue
		}
		pending++

		yellow.Printf("  → Checking %s... ", blockchain.ShortHash(record.RootHash))

		upgraded_record, err := blockchain.UpgradeProof(&record)
		if err != nil {
			yellow.Println("still pending.")
			continue
		}

		if upgraded_record.Status == "confirmed" {
			green.Println("CONFIRMED on Bitcoin! ✓")
			upgraded++
		}
	}

	if pending == 0 {
		green.Println("  All proofs already confirmed!")
	} else {
		fmt.Println()
		fmt.Printf("  Checked: %d pending  |  Upgraded: %d\n", pending, upgraded)
		if upgraded < pending {
			yellow.Println("  Remaining proofs are still waiting for Bitcoin confirmation.")
			fmt.Println("  Bitcoin blocks are mined ~every 10 minutes.")
			fmt.Println("  Full confirmation (6 blocks) takes ~1 hour.")
		}
	}

	fmt.Println()
	return nil
}

// ── sentinel proof verify ─────────────────────────────────────────────────────

var proofVerifyCmd = &cobra.Command{
	Use:   "verify <root-hash>",
	Short: "Verify a specific hash has a valid proof",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runProofVerify(args[0])
	},
}

func runProofVerify(rootHash string) error {
	bold := color.New(color.Bold)
	green := color.New(color.FgGreen, color.Bold)
	yellow := color.New(color.FgYellow)
	red := color.New(color.FgRed)

	fmt.Println()
	bold.Println("  SENTINEL PROOF — Verify Hash")
	fmt.Println()

	fmt.Printf("  Looking up: %s\n\n", blockchain.ShortHash(rootHash))

	record, err := blockchain.VerifyHash(rootHash)
	if err != nil {
		red.Printf("  ✗ No proof found for this hash.\n")
		fmt.Println("  Either this hash was never anchored, or the proof index is missing.")
		fmt.Println()
		return nil
	}

	green.Println("  ✓ Proof found!")
	fmt.Println()
	fmt.Printf("  Root Hash:   %s\n", record.RootHash)
	fmt.Printf("  Submitted:   %s\n", record.SubmittedAt.Format(time.RFC1123))
	fmt.Printf("  .ots file:   %s\n", record.OTSFile)

	switch record.Status {
	case "confirmed":
		green.Printf("  Status:      ✓ CONFIRMED on Bitcoin\n")
		if record.BitcoinTx != "" {
			fmt.Printf("  Bitcoin TX:  %s\n", record.BitcoinTx)
		}
	case "pending":
		yellow.Printf("  Status:      ⏳ PENDING — run 'sentinel proof upgrade'\n")
	default:
		red.Printf("  Status:      ✗ %s\n", record.Status)
	}

	// Verify the .ots file itself hasn't been tampered with
	otsHash, err := blockchain.HashOTSFile(record.OTSFile)
	if err != nil {
		red.Println("\n  ⚠  Warning: .ots file not found or unreadable!")
	} else {
		fmt.Printf("\n  .ots SHA-256: %s\n", otsHash)
		green.Println("  .ots file integrity: OK")
	}

	fmt.Println()
	return nil
}
