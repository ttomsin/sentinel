package cmd

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "sentinel",
	Short: "Sentinel — Protect your code from AI training",
	Long: color.New(color.FgYellow, color.Bold).Sprint(`
 ███████╗███████╗███╗   ██╗████████╗██╗███╗   ██╗███████╗██╗
 ██╔════╝██╔════╝████╗  ██║╚══██╔══╝██║████╗  ██║██╔════╝██║
 ███████╗█████╗  ██╔██╗ ██║   ██║   ██║██╔██╗ ██║█████╗  ██║
 ╚════██║██╔══╝  ██║╚██╗██║   ██║   ██║██║╚██╗██║██╔══╝  ██║
 ███████║███████╗██║ ╚████║   ██║   ██║██║ ╚████║███████╗███████╗
 ╚══════╝╚══════╝╚═╝  ╚═══╝   ╚═╝   ╚═╝╚═╝  ╚═══╝╚══════╝╚══════╝
`) + `
 Your code. Your rights. Protected.

 Sentinel sits on top of Git and gives your code three layers of protection:
   01  PREVENT  — Encrypts code before it ever reaches GitHub
   02  DETECT   — Scans AI outputs for similarity to your work
   03  PROVE    — Blockchain-anchored proof of authorship

 Run 'sentinel init' to get started in any Git repository.`,
	// Don't show usage on every error
	SilenceUsage: true,
}

// Execute is called by main.go — runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Register all subcommands
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(keygenCmd)
	rootCmd.AddCommand(commitCmd)
	rootCmd.AddCommand(pushCmd)
	rootCmd.AddCommand(pullCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(grantCmd)
	rootCmd.AddCommand(revokeCmd)
	rootCmd.AddCommand(whohasCmd)
	rootCmd.AddCommand(collabCmd)
	rootCmd.AddCommand(scanCmd)
	rootCmd.AddCommand(proofCmd)
	rootCmd.AddCommand(logCmd)
}
