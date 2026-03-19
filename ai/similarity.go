package ai

import (
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"strings"
)

// SimilarityResult holds the result of comparing two pieces of code
type SimilarityResult struct {
	Score           int    // 0-100
	EvidenceWeight  string // "strong", "moderate", "weak", "none"
	StructuralMatch int    // AST structure similarity 0-100
	TokenMatch      int    // token-level similarity 0-100
	Details         string
}

// AnalyseSimilarity compares original code to AI output
// Uses multiple layers: token similarity + AST structure
func AnalyseSimilarity(original, aiOutput string) SimilarityResult {
	// Layer 1: Token-level similarity (surface)
	tokenScore := tokenSimilarity(original, aiOutput)

	// Layer 2: AST structural similarity (deep)
	structScore := astSimilarity(original, aiOutput)

	// Weighted combination:
	// Structure matters more than surface tokens
	// because AI can easily rename variables
	combined := int(math.Round(float64(tokenScore)*0.35 + float64(structScore)*0.65))

	weight := evidenceWeight(combined)
	details := buildDetails(tokenScore, structScore, combined)

	return SimilarityResult{
		Score:           combined,
		EvidenceWeight:  weight,
		StructuralMatch: structScore,
		TokenMatch:      tokenScore,
		Details:         details,
	}
}

// tokenSimilarity computes word/token overlap between two code strings
// This is the "surface" layer — catches same variable names, same keywords
func tokenSimilarity(a, b string) int {
	tokensA := tokenize(a)
	tokensB := tokenize(b)

	if len(tokensA) == 0 || len(tokensB) == 0 {
		return 0
	}

	setA := make(map[string]int)
	for _, t := range tokensA {
		setA[t]++
	}

	setB := make(map[string]int)
	for _, t := range tokensB {
		setB[t]++
	}

	// Compute intersection size
	intersection := 0
	for token, countA := range setA {
		if countB, ok := setB[token]; ok {
			if countA < countB {
				intersection += countA
			} else {
				intersection += countB
			}
		}
	}

	// Union size
	union := len(tokensA) + len(tokensB) - intersection

	if union == 0 {
		return 0
	}

	score := int(math.Round(float64(intersection) / float64(union) * 100))

	// Boost score for exact identifier matches (variable names, function names)
	// These are strong evidence — AI rarely chooses the same names by coincidence
	identMatches := countIdentifierMatches(tokensA, tokensB)
	if identMatches > 3 {
		score = min(score+15, 100)
	}

	return score
}

// astSimilarity compares the structural shape of two Go functions
// This is the "deep" layer — catches same logic even with different names
func astSimilarity(original, aiOutput string) int {
	// Extract just the code from AI response (remove markdown, explanation text)
	cleanAI := extractCodeBlock(aiOutput)

	sigA := extractASTSignature(wrapInPackage(original))
	sigB := extractASTSignature(wrapInPackage(cleanAI))

	if sigA == "" || sigB == "" {
		// Fallback to enhanced token similarity if AST parsing fails
		return tokenSimilarity(original, cleanAI)
	}

	return tokenSimilarity(sigA, sigB)
}

// extractASTSignature walks the AST and produces a normalized structural signature
// Variable names are replaced with TYPE placeholders so renaming doesn't affect score
func extractASTSignature(src string) string {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", src, 0)
	if err != nil {
		return ""
	}

	var parts []string

	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			return false
		}

		switch node := n.(type) {

		// Control flow — these are very revealing
		case *ast.IfStmt:
			parts = append(parts, "IF")
		case *ast.ForStmt:
			parts = append(parts, "FOR")
		case *ast.RangeStmt:
			parts = append(parts, "RANGE")
		case *ast.SwitchStmt:
			parts = append(parts, "SWITCH")
		case *ast.SelectStmt:
			parts = append(parts, "SELECT")
		case *ast.ReturnStmt:
			parts = append(parts, "RETURN")
		case *ast.DeferStmt:
			parts = append(parts, "DEFER")
		case *ast.GoStmt:
			parts = append(parts, "GO")
		case *ast.SendStmt:
			parts = append(parts, "SEND")

		// Operations
		case *ast.AssignStmt:
			if node.Tok.String() == ":=" {
				parts = append(parts, "DECLARE")
			} else {
				parts = append(parts, "ASSIGN")
			}
		case *ast.BinaryExpr:
			parts = append(parts, "BINOP:"+node.Op.String())
		case *ast.UnaryExpr:
			parts = append(parts, "UNOP:"+node.Op.String())

		// Type information — keep these as they reveal design choices
		case *ast.ArrayType:
			parts = append(parts, "ARRAY")
		case *ast.MapType:
			parts = append(parts, "MAP")
		case *ast.ChanType:
			parts = append(parts, "CHAN")
		case *ast.InterfaceType:
			parts = append(parts, "INTERFACE")
		case *ast.StructType:
			parts = append(parts, "STRUCT")

		// Function calls — method names are revealing
		case *ast.CallExpr:
			if sel, ok := node.Fun.(*ast.SelectorExpr); ok {
				// Keep method names — they reveal API usage patterns
				parts = append(parts, "CALL:"+sel.Sel.Name)
			}

		// Literals — their types are meaningful, not their values
		case *ast.BasicLit:
			parts = append(parts, "LIT:"+node.Kind.String())
		}

		return true
	})

	return strings.Join(parts, " ")
}

// extractCodeBlock strips markdown fences and prose from AI responses
// AI models often wrap code in ```go ... ``` blocks
func extractCodeBlock(response string) string {
	// Try to find a code block
	if idx := strings.Index(response, "```go"); idx != -1 {
		start := idx + 5
		end := strings.Index(response[start:], "```")
		if end != -1 {
			return strings.TrimSpace(response[start : start+end])
		}
	}

	if idx := strings.Index(response, "```"); idx != -1 {
		start := idx + 3
		// Skip language identifier line
		if nl := strings.Index(response[start:], "\n"); nl != -1 {
			start += nl + 1
		}
		end := strings.Index(response[start:], "```")
		if end != -1 {
			return strings.TrimSpace(response[start : start+end])
		}
	}

	// No code block found — try to extract just the function
	lines := strings.Split(response, "\n")
	var codeLines []string
	inCode := false

	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "func ") {
			inCode = true
		}
		if inCode {
			codeLines = append(codeLines, line)
		}
	}

	if len(codeLines) > 0 {
		return strings.Join(codeLines, "\n")
	}

	return response
}

// wrapInPackage wraps a bare function in a package declaration so Go's parser accepts it
func wrapInPackage(code string) string {
	if !strings.Contains(code, "package ") {
		return "package main\n\n" + code
	}
	return code
}

// tokenize splits code into meaningful tokens, filtering noise
func tokenize(code string) []string {
	// Replace common separators with spaces
	replacer := strings.NewReplacer(
		"(", " ", ")", " ",
		"{", " ", "}", " ",
		"[", " ", "]", " ",
		",", " ", ";", " ",
		".", " ", ":", " ",
		"\t", " ", "\n", " ",
		"\"", " ", "'", " ",
	)

	cleaned := replacer.Replace(code)

	var tokens []string
	for _, t := range strings.Fields(cleaned) {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		// Skip very common Go keywords that appear everywhere
		switch t {
		case "func", "return", "if", "for", "range",
			"err", "nil", "true", "false", "var",
			"const", "type", "import", "package":
			continue
		}
		// Skip single characters
		if len(t) <= 1 {
			continue
		}
		tokens = append(tokens, strings.ToLower(t))
	}

	return tokens
}

// countIdentifierMatches counts matching identifiers between two token lists
// Matching specific identifiers (variable names, method names) is strong evidence
func countIdentifierMatches(a, b []string) int {
	setA := make(map[string]bool)
	for _, t := range a {
		if len(t) > 3 { // only meaningful identifiers
			setA[t] = true
		}
	}

	count := 0
	for _, t := range b {
		if len(t) > 3 && setA[t] {
			count++
		}
	}
	return count
}

// evidenceWeight converts a similarity score to a legal evidence weight
func evidenceWeight(score int) string {
	switch {
	case score >= 75:
		return "strong"
	case score >= 50:
		return "moderate"
	case score >= 25:
		return "weak"
	default:
		return "none"
	}
}

// buildDetails generates a human-readable explanation of the similarity result
func buildDetails(tokenScore, structScore, combined int) string {
	var lines []string

	lines = append(lines, "Analysis breakdown:")

	if structScore >= 70 {
		lines = append(lines, "  • High structural similarity — AI reproduced the same logical structure")
	} else if structScore >= 40 {
		lines = append(lines, "  • Moderate structural similarity — similar logical patterns detected")
	} else {
		lines = append(lines, "  • Low structural similarity — different implementation approach")
	}

	if tokenScore >= 60 {
		lines = append(lines, "  • High token similarity — matching identifiers and method names")
	} else if tokenScore >= 30 {
		lines = append(lines, "  • Moderate token similarity — some matching patterns")
	}

	switch {
	case combined >= 75:
		lines = append(lines, "  → STRONG EVIDENCE: Similarity is high enough to suggest training data exposure")
	case combined >= 50:
		lines = append(lines, "  → MODERATE EVIDENCE: Warrants further investigation")
	case combined >= 25:
		lines = append(lines, "  → WEAK EVIDENCE: Could be coincidental — common patterns")
	default:
		lines = append(lines, "  → NO EVIDENCE: AI implemented this differently")
	}

	return strings.Join(lines, "\n")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}