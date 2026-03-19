package ai

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// Probe is a single interrogation prompt targeting a specific function/pattern
type Probe struct {
	TargetFile     string // which file this probe is about
	TargetFunction string // which function we're probing
	Prompt         string // the actual prompt sent to the AI
	Signature      string // the function signature
	BodyHash       string // hash of the function body (for comparison)
	UniquenessScore int   // 0-100: how unique is this implementation?
}

// ProbeResult holds the AI's response to a probe
type ProbeResult struct {
	Probe          Probe
	AIResponse     string
	SimilarityScore int    // 0-100: how similar is AI output to original?
	EvidenceWeight  string // "strong", "moderate", "weak"
	Details         string // human-readable explanation
}

// GenerateProbes analyzes source files and creates interrogation prompts
// The goal: ask the AI to implement the same thing WITHOUT showing it our code
// If it reproduces our specific implementation — that's evidence it trained on us
func GenerateProbes(files []string) ([]Probe, error) {
	var probes []Probe

	for _, file := range files {
		// Only analyze Go files for now (Phase 4 can extend to other languages)
		if !strings.HasSuffix(file, ".go") {
			continue
		}

		fileProbes, err := probesFromGoFile(file)
		if err != nil {
			// Skip files that can't be parsed (encrypted, generated, etc.)
			continue
		}

		probes = append(probes, fileProbes...)
	}

	if len(probes) == 0 {
		return nil, fmt.Errorf("no probeable functions found in the provided files")
	}

	return probes, nil
}

// probesFromGoFile parses a Go source file and generates probes for its functions
func probesFromGoFile(path string) ([]Probe, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}

	var probes []Probe

	// Walk the AST and extract functions
	ast.Inspect(file, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}

		// Skip very short functions (too common to be meaningful)
		if fn.Body == nil || len(fn.Body.List) < 3 {
			return true
		}

		// Skip test functions
		if strings.HasPrefix(fn.Name.Name, "Test") ||
			strings.HasPrefix(fn.Name.Name, "Benchmark") {
			return true
		}

		probe := buildProbe(path, fn, src, fset)
		if probe != nil {
			probes = append(probes, *probe)
		}

		return true
	})

	return probes, nil
}

// buildProbe creates an interrogation probe for a specific function
func buildProbe(path string, fn *ast.FuncDecl, src []byte, fset *token.FileSet) *Probe {
	// Extract the function signature
	sig := extractSignature(fn, src, fset)

	// Extract what the function does conceptually (without showing the code)
	description := describeFunction(fn, src, fset)
	if description == "" {
		return nil
	}

	// Score uniqueness — how unusual is this implementation?
	uniqueness := scoreUniqueness(fn)

	// Skip very low uniqueness — too common to be meaningful evidence
	if uniqueness < 20 {
		return nil
	}

	// Build the probe prompt — ask AI to implement without seeing our code
	prompt := buildProbePrompt(fn.Name.Name, description, filepath.Base(path))

	return &Probe{
		TargetFile:      path,
		TargetFunction:  fn.Name.Name,
		Prompt:          prompt,
		Signature:       sig,
		UniquenessScore: uniqueness,
	}
}

// buildProbePrompt creates a natural language prompt that asks the AI
// to implement something without referencing our specific code
func buildProbePrompt(funcName, description, filename string) string {
	return fmt.Sprintf(`You are a Go developer. Write a complete, idiomatic Go function called "%s" that does the following:

%s

Requirements:
- Write only the function implementation, nothing else
- Use idiomatic Go patterns  
- Include proper error handling where appropriate
- Do not add comments explaining what you're doing

Write only the Go code:`, funcName, description)
}

// describeFunction generates a natural language description of what a function does
// by analyzing its AST — WITHOUT including the actual implementation details
func describeFunction(fn *ast.FuncDecl, src []byte, fset *token.FileSet) string {
	var parts []string

	// What does it take as input?
	if fn.Type.Params != nil && len(fn.Type.Params.List) > 0 {
		params := describeParams(fn.Type.Params.List)
		if params != "" {
			parts = append(parts, "Takes "+params+" as input")
		}
	}

	// What does it return?
	if fn.Type.Results != nil && len(fn.Type.Results.List) > 0 {
		results := describeResults(fn.Type.Results.List)
		if results != "" {
			parts = append(parts, "Returns "+results)
		}
	}

	// What operations does it perform? (inferred from AST)
	ops := inferOperations(fn)
	parts = append(parts, ops...)

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, "\n- ")
}

// inferOperations analyzes the function body AST to describe what it does
// without revealing the specific implementation
func inferOperations(fn *ast.FuncDecl) []string {
	var ops []string
	seen := map[string]bool{}

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch node := n.(type) {

		case *ast.RangeStmt:
			if !seen["range"] {
				ops = append(ops, "Iterates over a collection")
				seen["range"] = true
			}

		case *ast.IfStmt:
			if !seen["if"] {
				ops = append(ops, "Contains conditional logic with error checking")
				seen["if"] = true
			}

		case *ast.ReturnStmt:
			// Already handled in describeResults

		case *ast.GoStmt:
			if !seen["goroutine"] {
				ops = append(ops, "Spawns concurrent goroutines")
				seen["goroutine"] = true
			}

		case *ast.SelectStmt:
			if !seen["select"] {
				ops = append(ops, "Uses channel selection for concurrency")
				seen["select"] = true
			}

		case *ast.DeferStmt:
			if !seen["defer"] {
				ops = append(ops, "Uses deferred cleanup")
				seen["defer"] = true
			}

		case *ast.CallExpr:
			// Look for specific notable calls
			if sel, ok := node.Fun.(*ast.SelectorExpr); ok {
				callName := sel.Sel.Name
				switch {
				case callName == "Errorf" || callName == "Wrap":
					if !seen["errwrap"] {
						ops = append(ops, "Wraps errors with context")
						seen["errwrap"] = true
					}
				case callName == "WriteFile" || callName == "ReadFile":
					if !seen["fileio"] {
						ops = append(ops, "Performs file I/O operations")
						seen["fileio"] = true
					}
				case callName == "MarshalIndent" || callName == "Marshal":
					if !seen["json"] {
						ops = append(ops, "Serializes data to JSON")
						seen["json"] = true
					}
				case callName == "Unmarshal":
					if !seen["jsonparse"] {
						ops = append(ops, "Parses JSON data")
						seen["jsonparse"] = true
					}
				}
			}
		}
		return true
	})

	return ops
}

// describeParams generates a natural language description of function parameters
func describeParams(fields []*ast.Field) string {
	var params []string
	for _, field := range fields {
		typeStr := exprToString(field.Type)
		if typeStr != "" {
			params = append(params, typeStr)
		}
	}
	if len(params) == 0 {
		return ""
	}
	return strings.Join(params, ", ")
}

// describeResults describes return types in natural language
func describeResults(fields []*ast.Field) string {
	var results []string
	for _, field := range fields {
		typeStr := exprToString(field.Type)
		if typeStr != "" {
			results = append(results, typeStr)
		}
	}
	return strings.Join(results, ", ")
}

// exprToString converts an AST expression to a string representation
func exprToString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + exprToString(e.X)
	case *ast.ArrayType:
		return "[]" + exprToString(e.Elt)
	case *ast.SelectorExpr:
		return exprToString(e.X) + "." + e.Sel.Name
	case *ast.MapType:
		return "map[" + exprToString(e.Key) + "]" + exprToString(e.Value)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.Ellipsis:
		return "..." + exprToString(e.Elt)
	}
	return ""
}

// extractSignature extracts the function signature as a string
func extractSignature(fn *ast.FuncDecl, src []byte, fset *token.FileSet) string {
	start := fset.Position(fn.Pos()).Offset
	end := fset.Position(fn.Body.Lbrace).Offset
	if start >= 0 && end <= len(src) {
		return strings.TrimSpace(string(src[start:end]))
	}
	return fn.Name.Name
}

// scoreUniqueness estimates how unique a function implementation is (0-100)
// Higher score = more unique = stronger evidence if AI reproduces it
func scoreUniqueness(fn *ast.FuncDecl) int {
	score := 30 // base score

	// More statements = more complex = more unique
	if fn.Body != nil {
		stmtCount := countStatements(fn.Body)
		if stmtCount > 5 {
			score += 10
		}
		if stmtCount > 10 {
			score += 15
		}
		if stmtCount > 20 {
			score += 20
		}
	}

	// Multiple return values = more Go-specific pattern
	if fn.Type.Results != nil && len(fn.Type.Results.List) > 1 {
		score += 10
	}

	// Has receiver (method on struct) = more specific
	if fn.Recv != nil {
		score += 5
	}

	// Contains goroutines or channels = more unique pattern
	hasGoroutine := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if _, ok := n.(*ast.GoStmt); ok {
			hasGoroutine = true
		}
		return !hasGoroutine
	})
	if hasGoroutine {
		score += 15
	}

	if score > 100 {
		score = 100
	}
	return score
}

// countStatements counts the number of statements in a function body
func countStatements(body *ast.BlockStmt) int {
	count := 0
	ast.Inspect(body, func(n ast.Node) bool {
		if _, ok := n.(ast.Stmt); ok {
			count++
		}
		return true
	})
	return count
}