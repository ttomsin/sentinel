# How Phase 4 Works — The AI Interrogation Layer

Phase 4 is Sentinel's most sophisticated feature. It doesn't just compare code — it **interrogates AI models** to measure how well they know your specific implementation. This document explains every step from top to bottom.

---

## The Core Idea

Most people think about AI code similarity the wrong way:

> ❌ "Does AI output look like my code?"

That's the wrong question. AI is creative — it can write the same logic in a hundred different ways. Surface similarity alone is weak evidence.

The right question is:

> ✅ "Does AI know the specific way I implemented something?"

There is a profound difference. Let's use a real example.

---

## A Real Example — JWT Token Validation

Suppose you wrote a JWT validation function. There are dozens of ways to implement JWT validation in Go. Different libraries, different error handling styles, different claim structures, different expiry check approaches.

When Sentinel ran against a real Go project, it got:

```
[1/198] Probing: ValidateJwtToken → 61% similarity [moderate evidence]
[2/198] Probing: GenerateJwtToken → 66% similarity [moderate evidence]
```

**What does 66% mean?**

It means when Gemini was asked "write a JWT generation function in Go" — without being shown the original code — it produced something with 66% structural and token similarity to the specific implementation in that codebase.

That's not AI being "creative." That's AI recalling a pattern it absorbed during training.

---

## The Full Pipeline — Step by Step

### Step 1 — Source File Collection

```
sentinel scan run
        ↓
git ls-files → list of all tracked source files
        ↓
Filter: .go files only (Phase 4, more languages in future)
        ↓
Skip: test files, generated files, binaries
```

### Step 2 — AST Analysis (Abstract Syntax Tree)

Before generating any probes, Sentinel parses every source file using Go's built-in `go/ast` and `go/parser` packages. This converts your code from text into a **tree structure** that represents what the code actually does.

```go
// Your code (text)
func ValidateJwtToken(tokenStr string) (*Claims, error) {
    token, err := jwt.ParseWithClaims(...)
    if err != nil {
        return nil, fmt.Errorf("invalid token: %w", err)
    }
    ...
}

// What AST sees (structure, not text)
FuncDecl: ValidateJwtToken
  Params: [string]
  Returns: [*Claims, error]
  Body:
    AssignStmt (declare)
      CallExpr: ParseWithClaims
    IfStmt
      CallExpr: Errorf (error wrapping)
    ReturnStmt
```

The AST doesn't care about variable names or formatting. It sees the **logical shape** of your code.

### Step 3 — Probe Generation

For each function that passes the uniqueness threshold, Sentinel generates a **probe prompt** — a natural language description of what the function does, without revealing HOW you implemented it.

```
Original function: ValidateJwtToken

Sentinel extracts:
  - Takes: string (the token)
  - Returns: *Claims, error
  - Operations: parses JWT, validates claims, wraps errors, uses defer

Generated probe prompt:
  "You are a Go developer. Write a complete, idiomatic Go function
   called 'ValidateJwtToken' that does the following:

   Takes string as input
   Returns *Claims, error
   Parses JWT with claims validation
   Contains conditional logic with error checking
   Wraps errors with context
   Uses deferred cleanup

   Write only the Go code:"
```

Notice what's NOT in the prompt:
- Your actual code
- Which JWT library you used
- Your specific error messages
- Your variable names
- Your claim structure

The AI has to produce this from its own knowledge. If it produces something similar to yours — that knowledge came from somewhere.

### Step 4 — AI Interrogation

Sentinel sends each probe to your configured AI provider:

```
198 probes → AI provider (OpenAI / Anthropic / Gemini / Ollama)
                    ↓
             198 AI responses
```

Each response is the AI's attempt to implement the same function from scratch.

### Step 5 — Two-Layer Similarity Analysis

This is where the legal value is built. Sentinel compares your original code to the AI's response using **two independent layers**:

#### Layer 1 — Token Similarity (Surface, 35% weight)

Compares the actual words, identifiers, and method names between the two implementations.

```
Your code uses:     jwt.ParseWithClaims, Claims, keyFunc, ValidationError
AI output uses:     jwt.ParseWithClaims, Claims, keyFunc, ValidationError
                    ↑ same library calls, same type names
Token similarity:   HIGH
```

Matching specific identifier names is significant. There are hundreds of JWT libraries and thousands of possible variable names. The probability of independently choosing the same ones decreases exponentially with each match.

#### Layer 2 — Structural Similarity (AST Shape, 65% weight)

This layer is name-independent. Sentinel converts BOTH implementations to their AST signatures — normalized trees where all variable names are replaced with type placeholders.

```
Your AST signature:
  DECLARE CALL:ParseWithClaims IF CALL:Errorf RETURN DECLARE RANGE IF RETURN

AI AST signature:
  DECLARE CALL:ParseWithClaims IF CALL:Errorf RETURN DECLARE RANGE IF RETURN

Structural match: IDENTICAL
```

Even if the AI renamed every variable, used different error messages, and formatted the code differently — the structural signature catches it. The logical shape of your implementation is preserved.

Structural similarity has 65% weight because it's **much harder to evade** than token similarity. An AI can accidentally use the same variable names, but reproducing the exact logical structure of an unusual implementation is statistically meaningful.

### Step 6 — Uniqueness Scoring

Not all similarity is equal. Finding 70% similarity on a bubble sort is meaningless — everyone writes bubble sorts the same way. Finding 70% similarity on a custom retry mechanism with exponential backoff is very significant.

Sentinel scores each function's uniqueness (0-100):

```
Factors that increase uniqueness:
  + More statements (complex logic = more distinctive)
  + Multiple return values (Go-specific patterns)
  + Goroutines or channels (concurrent patterns)
  + Method receivers (struct-specific behavior)
  + File I/O, JSON handling, network calls (domain-specific)

Factors that decrease uniqueness:
  - Very short functions (< 3 statements)
  - Common patterns everyone writes
  - Standard library boilerplate
```

The higher the uniqueness score, the more damning a high similarity result becomes.

### Step 7 — Evidence Classification

Sentinel combines similarity score and uniqueness into a legal evidence weight:

```
Score ≥ 75%  →  STRONG EVIDENCE
Score ≥ 50%  →  MODERATE EVIDENCE
Score ≥ 25%  →  WEAK EVIDENCE
Score < 25%  →  NO EVIDENCE
```

These aren't arbitrary thresholds. They're calibrated against the probability of independent invention:

```
At 75%+ structural similarity on a unique function:
The probability that an AI independently arrived at the same
implementation is statistically negligible.

At 50-75%:
Warrants investigation. Could be common patterns, could be training.
Best combined with other evidence.

Below 25%:
AI implemented this differently. No useful signal.
```

### Step 8 — Report Generation

All results are saved to `.sentinel/scan_<timestamp>.json` and displayed in the terminal:

```
── STRONG EVIDENCE (2) ──

  func ValidateJwtToken
  File:       internal/auth/jwt.go
  Similarity: 78%  (structural: 82%, tokens: 71%)
  Uniqueness: 74/100

  func BuildRetryMiddleware
  File:       internal/http/middleware.go
  Similarity: 81%  (structural: 85%, tokens: 74%)
  Uniqueness: 88/100

── MODERATE EVIDENCE (5) ──
  ...

── NO EVIDENCE (191 functions) ──
```

---

## What the Numbers Mean Legally

The scan results become evidence when combined with the rest of Sentinel's stack:

```
BLOCKCHAIN PROOF (Phase 3):
"My code existed on [date] — before your training run"
                    +
SIMILARITY SCAN (Phase 4):
"Your model reproduces my specific implementation at 78% similarity"
                    +
UNIQUENESS SCORE (Phase 4):
"That function scores 74/100 for uniqueness —
 independent invention is statistically implausible"
                    =
A legal argument. Not a guarantee — but an argument
that requires an explanation from the AI company.
```

---

## Why 198 Probes is Good

198 probes means Sentinel found 198 functions worth interrogating. That's 198 individual data points. Even if most come back with low similarity, the ones that don't stand out sharply.

In the real test run:
```
ValidateJwtToken   → 61% [moderate]
GenerateJwtToken   → 66% [moderate]
```

Both JWT functions showed moderate evidence. That's a pattern — not a fluke. When multiple related functions in the same domain show elevated similarity, it suggests the AI absorbed an entire module, not just a coincidental function.

---

## Commands Reference

```bash
# Configure AI provider
sentinel scan config --provider gemini --key AIza... --model gemini-2.0-flash
sentinel scan config --provider openai --key sk-...
sentinel scan config --provider anthropic --key sk-ant-...
sentinel scan config --provider ollama        # free, local

# List available models
sentinel scan models

# Run interrogation (all files)
sentinel scan run

# Run on specific files only
sentinel scan run --files internal/auth/jwt.go,internal/http/middleware.go

# View latest report
sentinel scan report
```

---

## Technical Notes

- Probe generation uses Go's standard `go/ast` and `go/parser` — no external dependencies
- AI HTTP calls use Go's standard `net/http` — no SDK dependencies
- Supports 4 providers: OpenAI, Anthropic, Google Gemini, Ollama (local/free)
- All results stored in `.sentinel/scan_<timestamp>.json`
- Scan reports travel with the repo — your evidence history is always present
- 198 probes against Gemini takes approximately 8-15 minutes depending on rate limits
