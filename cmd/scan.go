package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/sentinel-cli/sentinel/ai"
	"github.com/sentinel-cli/sentinel/git"
	"github.com/spf13/cobra"
)

// scanCmd is now a parent command with subcommands
var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "AI interrogation — detect if AI models trained on your code",
	Long: `Interrogate AI models to detect if they trained on your code.

How it works:
  1. Analyses your source files to extract function signatures and patterns
  2. Generates probe prompts that ask AI to implement the same thing
  3. Queries the configured AI provider with those prompts
  4. Compares AI output to your implementation using AST structural analysis
  5. Scores similarity and generates an evidence report

Subcommands:
  sentinel scan config    Configure your AI provider and API key
  sentinel scan run       Run the interrogation against your codebase
  sentinel scan report    Show the latest scan report`,

	RunE: func(cmd *cobra.Command, args []string) error {
		return runScanRun(nil)
	},
}

func init() {
	scanCmd.AddCommand(scanConfigCmd)
	scanCmd.AddCommand(scanRunCmd)
	scanCmd.AddCommand(scanReportCmd)
	scanCmd.AddCommand(scanModelsCmd)
}

// ── sentinel scan config ──────────────────────────────────────────────────────

var scanConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Configure your AI provider and API key",
	Long: `Set up which AI provider Sentinel should interrogate.

Examples:
  sentinel scan config --provider openai --key sk-...
  sentinel scan config --provider anthropic --key sk-ant-...
  sentinel scan config --provider gemini --key AIza...
  sentinel scan config --provider ollama                           # free, local
  sentinel scan config --provider gemini --key AIza... --model gemini-2.0-flash
  sentinel scan config --provider openai --key sk-... --model gpt-4-turbo

The --model flag lets you specify exactly which model to use.
Run 'sentinel scan models' to see available models per provider.

Your API key is stored in .sentinel/ai_config.json (owner read-only).
You can also set: export SENTINEL_AI_KEY=your-key`,

	RunE: func(cmd *cobra.Command, args []string) error {
		provider, _ := cmd.Flags().GetString("provider")
		key, _ := cmd.Flags().GetString("key")
		model, _ := cmd.Flags().GetString("model")
		return runScanConfig(provider, key, model)
	},
}

func init() {
	scanConfigCmd.Flags().StringP("provider", "p", "", "AI provider: openai, anthropic, gemini, ollama")
	scanConfigCmd.Flags().StringP("key", "k", "", "Your API key")
	scanConfigCmd.Flags().StringP("model", "m", "", "Model name to use (overrides default — use 'sentinel scan models' to see options)")
}

func runScanConfig(provider, key, model string) error {
	bold := color.New(color.Bold)
	green := color.New(color.FgGreen, color.Bold)
	yellow := color.New(color.FgYellow)
	red := color.New(color.FgRed)

	fmt.Println()
	bold.Println("  SENTINEL SCAN — Configure AI Provider")
	fmt.Println()

	if provider == "" {
		fmt.Println("  Available providers:")
		fmt.Println()
		for p, info := range ai.ProviderInfo {
			freeTag := ""
			if info.Free {
				freeTag = color.New(color.FgGreen).Sprint(" (free)")
			}
			fmt.Printf("    %-12s %s%s\n", p, info.Name, freeTag)
			fmt.Printf("    %-12s Key format: %s\n\n", "", info.KeyHint)
		}
		fmt.Println("  Usage: sentinel scan config --provider <provider> --key <your-key>")
		fmt.Println()
		return nil
	}

	p := ai.Provider(strings.ToLower(provider))

	// Validate provider
	if _, ok := ai.ProviderInfo[p]; !ok {
		red.Printf("  ✗ Unknown provider: %s\n", provider)
		fmt.Println("  Supported: openai, anthropic, gemini, ollama")
		fmt.Println()
		return nil
	}

	// Ollama doesn't need a key
	if p != ai.ProviderOllama && key == "" {
		red.Printf("  ✗ API key required for %s\n", provider)
		fmt.Printf("  Usage: sentinel scan config --provider %s --key YOUR_KEY\n", provider)
		fmt.Println()
		return nil
	}

	// Apply default model
	if model == "" {
		model = ai.ProviderDefaults[p]
	}

	cfg := ai.Config{
		Provider: p,
		APIKey:   key,
		Model:    model,
	}

	yellow.Print("  → Saving configuration... ")
	if err := ai.SaveConfig(cfg); err != nil {
		red.Println("FAILED")
		return fmt.Errorf("failed to save config: %w", err)
	}
	green.Println("done.")

	fmt.Println()
	info := ai.ProviderInfo[p]
	green.Println("  ✓ AI provider configured!")
	fmt.Println()
	fmt.Printf("  Provider:  %s\n", info.Name)
	fmt.Printf("  Model:     %s\n", model)
	if key != "" {
		fmt.Printf("  Key:       %s\n", ai.MaskKey(key))
	} else {
		fmt.Println("  Key:       (not required for Ollama)")
	}
	fmt.Println()

	// Show other available models for this provider
	if examples, ok := ai.ProviderModelExamples[p]; ok {
		color.Cyan("  Other available models:")
		for _, m := range examples {
			if m == model {
				green.Printf("    ✓ %s  (current)\n", m)
			} else {
				fmt.Printf("      %s\n", m)
			}
		}
		fmt.Println()
		fmt.Printf("  To change model: sentinel scan config --provider %s --key <key> --model <model>\n", p)
	}

	fmt.Println()
	fmt.Println("  Run 'sentinel scan run' to start interrogating AI models.")
	fmt.Println()

	return nil
}

// ── sentinel scan models ─────────────────────────────────────────────────────

var scanModelsCmd = &cobra.Command{
	Use:   "models",
	Short: "List available models for each AI provider",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runScanModels()
	},
}

func runScanModels() error {
	bold := color.New(color.Bold)
	green := color.New(color.FgGreen)
	cyan := color.New(color.FgCyan)

	fmt.Println()
	bold.Println("  SENTINEL SCAN — Available Models")
	fmt.Println()
	fmt.Println("  Use: sentinel scan config --provider <p> --key <key> --model <model>")
	fmt.Println()

	for _, p := range []ai.Provider{ai.ProviderOpenAI, ai.ProviderAnthropic, ai.ProviderGemini, ai.ProviderOllama} {
		info := ai.ProviderInfo[p]
		freeTag := ""
		if info.Free {
			freeTag = green.Sprint("  (free)")
		}
		cyan.Printf("  %s%s\n", info.Name, freeTag)

		defaultModel := ai.ProviderDefaults[p]
		examples := ai.ProviderModelExamples[p]
		for _, m := range examples {
			if m == defaultModel {
				green.Printf("    ✓ %-35s (default)\n", m)
			} else {
				fmt.Printf("      %s\n", m)
			}
		}
		fmt.Println()
	}

	fmt.Println("  Note: AI providers release new models frequently.")
	fmt.Println("  Check your provider's docs for the latest model names.")
	fmt.Println()
	return nil
}

// ── sentinel scan run ─────────────────────────────────────────────────────────

var scanRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run AI interrogation against your codebase",
	RunE: func(cmd *cobra.Command, args []string) error {
		files, _ := cmd.Flags().GetStringSlice("files")
		return runScanRun(files)
	},
}

func init() {
	scanRunCmd.Flags().StringSliceP("files", "f", nil, "Specific files to scan (default: all tracked files)")
}

// ScanReport holds the full results of a scan session
type ScanReport struct {
	Timestamp    time.Time          `json:"timestamp"`
	Provider     string             `json:"provider"`
	Model        string             `json:"model"`
	FilesScanned int                `json:"files_scanned"`
	ProbesRun    int                `json:"probes_run"`
	Results      []ProbeReportEntry `json:"results"`
	Summary      ScanSummary        `json:"summary"`
}

type ProbeReportEntry struct {
	Function        string `json:"function"`
	File            string `json:"file"`
	UniquenessScore int    `json:"uniqueness_score"`
	SimilarityScore int    `json:"similarity_score"`
	EvidenceWeight  string `json:"evidence_weight"`
	StructuralMatch int    `json:"structural_match"`
	TokenMatch      int    `json:"token_match"`
	Details         string `json:"details"`
}

type ScanSummary struct {
	StrongEvidence   int `json:"strong_evidence"`
	ModerateEvidence int `json:"moderate_evidence"`
	WeakEvidence     int `json:"weak_evidence"`
	NoEvidence       int `json:"no_evidence"`
}

func runScanRun(specificFiles []string) error {
	bold := color.New(color.Bold)
	green := color.New(color.FgGreen, color.Bold)
	yellow := color.New(color.FgYellow)
	red := color.New(color.FgRed, color.Bold)
	cyan := color.New(color.FgCyan)

	fmt.Println()
	bold.Println("  SENTINEL SCAN — AI Interrogation")
	fmt.Println()

	// Step 1: Load AI client
	yellow.Print("  → Loading AI provider config... ")
	client, err := ai.NewClient()
	if err != nil {
		red.Println("NOT CONFIGURED")
		fmt.Println()
		fmt.Println("  " + err.Error())
		fmt.Println()
		return nil
	}
	green.Printf("using %s (%s)\n", client.ProviderName(), client.Model())

	// Step 2: Get files to scan
	yellow.Print("  → Collecting source files... ")
	var files []string
	if len(specificFiles) > 0 {
		files = specificFiles
	} else {
		files, err = git.GetTrackedFiles()
		if err != nil {
			red.Println("FAILED")
			return fmt.Errorf("failed to get tracked files: %w", err)
		}
	}
	green.Printf("%d files.\n", len(files))

	// Step 3: Generate probes
	yellow.Print("  → Analysing code and generating probes... ")
	probes, err := ai.GenerateProbes(files)
	if err != nil {
		red.Println("FAILED")
		fmt.Println()
		yellow.Println("  " + err.Error())
		fmt.Println()
		return nil
	}
	green.Printf("%d probes generated.\n", len(probes))

	fmt.Println()
	cyan.Printf("  Interrogating %s with %d probes...\n", client.ProviderName(), len(probes))
	fmt.Println()

	// Step 4: Run each probe
	var entries []ProbeReportEntry
	summary := ScanSummary{}

	for i, probe := range probes {
		fmt.Printf("  [%d/%d] Probing: %s", i+1, len(probes), probe.TargetFunction)

		// Query the AI
		response, err := client.Complete(probe.Prompt)
		if err != nil {
			red.Printf(" → ERROR: %v\n", err)
			continue
		}

		// Read original function source
		src, readErr := os.ReadFile(probe.TargetFile)
		originalCode := ""
		if readErr == nil {
			originalCode = string(src)
		}

		// Analyse similarity
		result := ai.AnalyseSimilarity(originalCode, response)

		// Colour the score
		scoreStr := fmt.Sprintf("%d%%", result.Score)
		switch result.EvidenceWeight {
		case "strong":
			red.Printf(" → %s similarity [%s evidence]\n", scoreStr, result.EvidenceWeight)
		case "moderate":
			yellow.Printf(" → %s similarity [%s evidence]\n", scoreStr, result.EvidenceWeight)
		case "weak":
			fmt.Printf(" → %s similarity [%s evidence]\n", scoreStr, result.EvidenceWeight)
		default:
			color.New(color.FgGreen).Printf(" → %s similarity [no evidence]\n", scoreStr)
		}

		// Track summary
		switch result.EvidenceWeight {
		case "strong":
			summary.StrongEvidence++
		case "moderate":
			summary.ModerateEvidence++
		case "weak":
			summary.WeakEvidence++
		default:
			summary.NoEvidence++
		}

		entries = append(entries, ProbeReportEntry{
			Function:        probe.TargetFunction,
			File:            probe.TargetFile,
			UniquenessScore: probe.UniquenessScore,
			SimilarityScore: result.Score,
			EvidenceWeight:  result.EvidenceWeight,
			StructuralMatch: result.StructuralMatch,
			TokenMatch:      result.TokenMatch,
			Details:         result.Details,
		})
	}

	// Step 5: Build and save report
	report := ScanReport{
		Timestamp:    time.Now().UTC(),
		Provider:     client.ProviderName(),
		Model:        client.Model(),
		FilesScanned: len(files),
		ProbesRun:    len(probes),
		Results:      entries,
		Summary:      summary,
	}

	reportFile := fmt.Sprintf(".sentinel/scan_%d.json", time.Now().Unix())
	reportData, _ := json.MarshalIndent(report, "", "  ")
	_ = os.WriteFile(reportFile, reportData, 0644)

	// Step 6: Print summary
	fmt.Println()
	bold.Println("  ─────────────────────────────────────")
	bold.Println("  SCAN COMPLETE — Evidence Summary")
	bold.Println("  ─────────────────────────────────────")
	fmt.Println()

	red.Printf("    Strong evidence:    %d function(s)\n", summary.StrongEvidence)
	yellow.Printf("    Moderate evidence:  %d function(s)\n", summary.ModerateEvidence)
	fmt.Printf("    Weak evidence:      %d function(s)\n", summary.WeakEvidence)
	color.New(color.FgGreen).Printf("    No evidence:        %d function(s)\n", summary.NoEvidence)

	fmt.Println()

	if summary.StrongEvidence > 0 || summary.ModerateEvidence > 0 {
		red.Println("  ⚠  Evidence of AI training detected.")
		fmt.Println("  Combined with your blockchain proof, you have grounds for legal action.")
		fmt.Println()
	} else {
		green.Println("  ✓ No significant evidence found in this scan.")
		fmt.Println()
	}

	cyan.Printf("  Full report saved: %s\n", reportFile)
	fmt.Println("  Run 'sentinel scan report' to view details.")
	fmt.Println()

	return nil
}

// ── sentinel scan report ──────────────────────────────────────────────────────

var scanReportCmd = &cobra.Command{
	Use:   "report",
	Short: "Show the latest scan report",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runScanReport()
	},
}

func runScanReport() error {
	bold := color.New(color.Bold)
	green := color.New(color.FgGreen)
	yellow := color.New(color.FgYellow)
	red := color.New(color.FgRed, color.Bold)
	cyan := color.New(color.FgCyan)

	// Find latest scan report
	entries, err := os.ReadDir(".sentinel")
	if err != nil {
		return fmt.Errorf("no .sentinel directory found")
	}

	latestReport := ""
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "scan_") && strings.HasSuffix(e.Name(), ".json") {
			latestReport = ".sentinel/" + e.Name()
		}
	}

	if latestReport == "" {
		yellow.Println("\n  No scan reports found. Run 'sentinel scan run' first.\n")
		return nil
	}

	data, err := os.ReadFile(latestReport)
	if err != nil {
		return fmt.Errorf("failed to read report: %w", err)
	}

	var report ScanReport
	if err := json.Unmarshal(data, &report); err != nil {
		return fmt.Errorf("corrupted report: %w", err)
	}

	fmt.Println()
	bold.Println("  SENTINEL SCAN REPORT")
	fmt.Println()
	fmt.Printf("  Date:     %s\n", report.Timestamp.Format(time.RFC1123))
	fmt.Printf("  Provider: %s (%s)\n", report.Provider, report.Model)
	fmt.Printf("  Files:    %d scanned, %d probes run\n", report.FilesScanned, report.ProbesRun)
	fmt.Println()

	// Print results grouped by evidence weight
	for _, weight := range []string{"strong", "moderate", "weak"} {
		var matches []ProbeReportEntry
		for _, r := range report.Results {
			if r.EvidenceWeight == weight {
				matches = append(matches, r)
			}
		}
		if len(matches) == 0 {
			continue
		}

		switch weight {
		case "strong":
			red.Printf("  ── STRONG EVIDENCE (%d) ──\n\n", len(matches))
		case "moderate":
			yellow.Printf("  ── MODERATE EVIDENCE (%d) ──\n\n", len(matches))
		case "weak":
			fmt.Printf("  ── WEAK EVIDENCE (%d) ──\n\n", len(matches))
		}

		for _, r := range matches {
			cyan.Printf("    func %s\n", r.Function)
			fmt.Printf("    File:       %s\n", r.File)
			fmt.Printf("    Similarity: %d%%  (structural: %d%%, tokens: %d%%)\n",
				r.SimilarityScore, r.StructuralMatch, r.TokenMatch)
			fmt.Printf("    Uniqueness: %d/100\n", r.UniquenessScore)
			fmt.Println()
		}
	}

	// Show clean functions
	clean := 0
	for _, r := range report.Results {
		if r.EvidenceWeight == "none" {
			clean++
		}
	}
	if clean > 0 {
		green.Printf("  ── NO EVIDENCE (%d functions) ──\n", clean)
		fmt.Println()
	}

	fmt.Printf("  Report file: %s\n", latestReport)
	fmt.Println()

	return nil
}
