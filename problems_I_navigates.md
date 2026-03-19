# Problems We Navigated — Building Sentinel

Building Sentinel was not a straight line. Every phase introduced a problem that looked simple until it wasn't. This document is an honest record of the hard problems we encountered and exactly how we solved them.

---

## Problem 1 — The Binary That Tried to Encrypt Itself

**Phase:** 2 (Encryption)

**What happened:**

The first real test of `sentinel commit` on the Sentinel codebase itself ended with this error:

```
failed to encrypt sentinel: open sentinel: text file busy
```

Sentinel was trying to encrypt its own compiled binary while that binary was still running. Linux prevents you from overwriting a running executable — hence "text file busy."

Beyond the binary, the file collection logic had no notion of what should and shouldn't be encrypted. It was collecting everything git tracked — including the compiled binary, `go.sum`, `sentinel.toml`, and `.gitignore`.

**How we solved it:**

Two fixes. First, a `defaultExcludes` map of files that should never be encrypted:

```go
var defaultExcludes = map[string]bool{
    "sentinel":      true,   // the compiled binary
    "sentinel.exe":  true,
    "sentinel.toml": true,
    "go.sum":        true,
    ".gitignore":    true,
}
```

Second, a `isBinaryFile()` function that checks if a file has no extension AND has the executable bit set — catching any compiled binary regardless of name:

```go
func isBinaryFile(path string) bool {
    if ext := filepath.Ext(path); ext != "" {
        return false  // has extension = source file
    }
    info, _ := os.Stat(path)
    return info.Mode()&0111 != 0  // executable bit = compiled binary
}
```

**Lesson:** When building a tool that operates on its own environment, you have to explicitly teach it what to skip. Implicit assumptions about "what a source file looks like" break immediately in practice.

---

## Problem 2 — The Encryption That Wasn't

**Phase:** 2 (Encryption)

**What happened:**

After fixing Problem 1, `sentinel commit` ran without errors and reported success. But when the code was pushed to GitHub and viewed in the browser — it was completely readable. Plaintext. No encryption at all.

The command output had said "Encrypting files (AES-256-GCM)... done." The encryption was genuinely happening. So why was GitHub showing plaintext?

**Root cause:**

Git's staging area is a snapshot. When you run `git add .`, Git takes a snapshot of your files at that moment. Any changes made to files on disk after that point are invisible to Git until you run `git add .` again.

The order was:

```
git add .       ← Git snapshots PLAINTEXT
encrypt files   ← Disk is now encrypted
git commit      ← Git commits the PLAINTEXT snapshot
decrypt files   ← Disk returns to normal
```

The encryption was real — the files on disk were correctly encrypted. But Git committed the pre-encryption snapshot. GitHub received plaintext.

**How we solved it:**

Add a second `git add .` after encryption and before commit:

```
git add .       ← Git snapshots plaintext
encrypt files   ← Disk encrypted
git add .       ← Git re-snapshots CIPHERTEXT ← THE FIX
git commit      ← Git commits the ciphertext ✓
decrypt files   ← Disk returns to normal
```

**Lesson:** Encryption on disk and encryption in Git's staging area are completely independent. You must explicitly tell Git about changes even if those changes are made by your own tool. This is the subtlest bug in the entire codebase and the hardest to spot without testing the full push-to-GitHub flow.

---

## Problem 3 — The Proof That Vanished

**Phase:** 3 (Blockchain)

**What happened:**

After `sentinel commit` ran successfully with blockchain anchoring enabled, running `sentinel proof status` immediately after returned:

```
No proofs found yet.
Run 'sentinel commit -m "message"' to create your first proof.
```

The commit had just happened. The anchoring was supposedly running. But no proof record existed.

**Root cause:**

The blockchain anchoring was launched as a Go goroutine — a background process running concurrently. The goroutine was supposed to make an HTTP call to OpenTimestamps, receive the `.ots` file, and save the proof record to disk.

The problem: Go kills all goroutines when the main process exits. The CLI command ran, launched the goroutine, printed the success message, and exited — all within milliseconds. The goroutine never had time to complete its HTTP call or save anything to disk.

```
sentinel commit runs
      ↓
goroutine launched (HTTP call to OpenTimestamps)
      ↓
main process prints success and EXITS
      ↓
all goroutines killed
      ↓
no proof saved
```

**How we solved it:**

Split the operation into two parts: a synchronous part that runs before the process exits, and an asynchronous part that can fail gracefully.

`RegisterProof()` — saves the proof record to disk **synchronously**, before any goroutine is launched. This records the root hash, hash file path, timestamp, and status "pending" immediately.

`AnchorHash()` — runs in a goroutine and makes the HTTP call. If it completes, it updates the record to include the `.ots` file data. If the process exits first, the record is already saved — just without the `.ots` file. The developer can run `sentinel proof upgrade` later.

```go
// Synchronous — must complete before process exits
record, err := blockchain.RegisterProof(rootHash, hashFile)

// Asynchronous — can fail gracefully
go func(rh, hf string) {
    _, _ = blockchain.AnchorHash(rh, hf)
}(rootHash, hashFile)
```

**Lesson:** Never rely on a goroutine to save state that the user will immediately query. Separate "record the intent" (synchronous) from "fulfil the intent" (async). This is a fundamental pattern in distributed systems that applies equally to CLI tools.

---

## Problem 4 — "Commit Failed" With No Explanation

**Phase:** 2 (Git wrapper)

**What happened:**

A developer edited a file, didn't run `git add .`, and ran `sentinel commit`. The result:

```
→ Running git commit... FAILED
Error: git commit failed: exit status 1
```

No information about what failed or why. `exit status 1` is git's generic error code for "something went wrong." The developer had no idea they needed to stage changes first.

**How we solved it:**

Two changes. First, auto-stage everything before committing — remove the requirement to manually run `git add .` entirely:

```go
// Step 3: Auto-stage all changes
if err := git.AddAll(); err != nil {
    return fmt.Errorf("git add failed: %w", err)
}
```

Second, add a pre-commit check for staged changes with a human-readable message:

```go
hasChanges, _ := git.HasStagedChanges()
if !hasChanges {
    yellow.Println("  Nothing to commit — no changes detected since last commit.")
    return nil
}
```

Third, add pattern matching on git's error output to give specific error messages for common failures:

```go
if strings.Contains(errStr, "Please tell me who you are") {
    return fmt.Errorf("git user not configured — run:\n" +
        "  git config --global user.email \"you@example.com\"\n" +
        "  git config --global user.name \"Your Name\"")
}
```

**Lesson:** A tool is only as good as its error messages. "exit status 1" tells a developer nothing. The extra 10 lines of error handling are worth more than 100 lines of feature code.

---

## Problem 5 — The Wrong Gemini Model

**Phase:** 4 (AI Interrogation)

**What happened:**

After configuring Gemini and running `sentinel scan run`, the error was:

```json
{
  "message": "models/gemini-1.5-flash is not found for API version v1beta,
   or is not supported for generateContent."
}
```

The model name `gemini-1.5-flash` was hardcoded as the default for Gemini — but Google had updated their API and that model was no longer available under the `v1beta` endpoint.

**How we solved it:**

Three changes. First, update the default to a current model:

```go
var ProviderDefaults = map[Provider]string{
    ProviderGemini: "gemini-2.0-flash",  // was gemini-1.5-flash
}
```

Second, add a `--model` flag to `sentinel scan config` so developers can specify any model name without waiting for a Sentinel update:

```bash
sentinel scan config --provider gemini --key AIza... --model gemini-2.5-flash
```

Third, add `sentinel scan models` — a command that lists known models per provider with the current default highlighted:

```
Google Gemini (free)
  ✓ gemini-2.0-flash    (default)
    gemini-1.5-pro
    gemini-2.0-pro-exp
```

**Lesson:** AI model names are not stable. Never hardcode a specific model as the only option. Build in override capability from the start, and document that models change. The `--model` flag is more important than any specific default.

---

## Problem 6 — The Encryption That Encrypted Too Much

**Phase:** 2 (Encryption)

**What happened:**

The first version of `GetTrackedFiles()` returned everything git was tracking. Running `sentinel commit` on Sentinel's own repo — 14 files — included the compiled binary `sentinel` (4.9MB). Beyond the binary problem (Problem 1), this also meant `sentinel.toml`, `go.sum`, `.gitignore`, and `README.md` were being encrypted.

Encrypting `README.md` means your GitHub repo shows a binary blob where your README should be. No description, no installation instructions, no project overview. The repo becomes useless as a public presence even though the source code is protected.

Encrypting `go.sum` means the Go toolchain can't verify dependencies. Encrypting `sentinel.toml` means Sentinel's own config becomes inaccessible.

**How we solved it:**

A multi-layer filtering system in `GetTrackedFiles()`:

```go
// Layer 1: Skip .sentinel/ internal directory always
if strings.HasPrefix(f, ".sentinel/") {
    continue
}

// Layer 2: Skip hardcoded non-source files
if defaultExcludes[filepath.Base(f)] {
    continue
}

// Layer 3: Skip executable binaries (no extension + executable bit)
if isBinaryFile(f) {
    continue
}

// Layer 4: Skip markdown and text files (keep READMEs human-readable)
if strings.HasSuffix(f, ".md") || strings.HasSuffix(f, ".txt") {
    continue
}
```

**Lesson:** "Everything git tracks" is not the same as "everything that should be encrypted." Encryption should target source code specifically — the thing AI trains on. Config files, dependency locks, documentation, and binaries have different requirements. Treat them differently.

---

## The Meta-Lesson

Looking across all six problems, a pattern emerges:

> **The bugs that matter most are not in the cryptography. They're in the integration.**

AES-256-GCM worked perfectly from day one. SHA-256 worked perfectly. HKDF worked perfectly. OpenTimestamps worked.

The bugs were in how pieces connected: the re-staging step, the goroutine lifecycle, the file filtering logic, the error messages. These are engineering problems, not cryptography problems.

This is true of security systems generally. The math is usually sound. The failure modes live at the boundaries — between systems, between steps, between what the code does and what the developer expects.

Building Sentinel required finding those boundaries and handling them explicitly. Every problem on this list was found through actual testing, not code review. There is no substitute for running the thing and seeing what breaks.
