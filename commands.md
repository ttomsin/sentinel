# Sentinel — Complete Command Reference

> Every command available in Sentinel, with usage, flags, and examples.

---

## Quick Reference

```
SETUP
  sentinel init                          Initialize Sentinel in a Git repo
  sentinel keygen                        Generate master encryption keys

DAILY WORKFLOW
  sentinel commit -m "message"           Hash, encrypt, commit, anchor to Bitcoin
  sentinel push                          Push encrypted code to remote
  sentinel pull                          Pull and auto-decrypt locally
  sentinel status                        Show protection status + git status
  sentinel log                           Sentinel-annotated commit history

PROOF (Phase 3)
  sentinel proof                         Show latest proof status
  sentinel proof status                  Show latest proof status
  sentinel proof list                    List all proof records
  sentinel proof upgrade                 Check Bitcoin confirmation on pending proofs
  sentinel proof verify <hash>           Verify a specific root hash

AI INTERROGATION (Phase 4)
  sentinel scan models                   List available models per provider
  sentinel scan config                   Configure AI provider
  sentinel scan run                      Run AI interrogation
  sentinel scan report                   View latest scan report

ACCESS CONTROL (Phase 5)
  sentinel grant <username>              Grant collaborator access
  sentinel revoke <username>             Revoke collaborator access
  sentinel revoke <username> --rotate    Hard revoke with key rotation
  sentinel whohas                        List all access records
  sentinel collab join --key <key>       Install key as collaborator
  sentinel collab status                 Check your access status
```

---

## Setup Commands

### `sentinel init`

Initialize Sentinel in the current directory.

```bash
sentinel init
```

**What it does:**
- Checks for an existing Git repository (runs `git init` if none found)
- Creates `.sentinel/` directory structure (keys, hashes, proofs)
- Adds `.sentinel/keys/` to `.gitignore` — keys are never committed
- Creates `sentinel.toml` config file

**Output directories created:**
```
.sentinel/keys/      ← your encryption keys (never committed)
.sentinel/hashes/    ← SHA-256 hash records per commit
.sentinel/proofs/    ← Bitcoin proof files
```

**Example:**
```bash
cd my-project
sentinel init
# → Initializing Sentinel...
# → Checking for Git repository... found.
# → Creating .sentinel/ directory... done.
# → Protecting keys in .gitignore... done.
# → Writing sentinel.toml config... done.
# ✓ Sentinel initialized successfully!
```

---

### `sentinel keygen`

Generate your master encryption key pair.

```bash
sentinel keygen
```

**What it does:**
- Generates a 256-bit AES key for encrypting source files
- Generates an Ed25519 key pair for signing proof certificates
- Saves all keys to `.sentinel/keys/` with `0600` permissions (owner-only)
- Displays your public key fingerprint

**Files created:**
```
.sentinel/keys/master.key    ← AES-256 encryption key (NEVER share)
.sentinel/keys/master.priv   ← Ed25519 private key (NEVER share)
.sentinel/keys/master.pub    ← Ed25519 public key (safe to share)
```

**Example:**
```bash
sentinel keygen
# → Generating AES-256 encryption key... done.
# → Generating Ed25519 signing key pair... done.
# → Saving keys to .sentinel/keys/... done.
# ✓ Keys generated successfully!
# Public Key Fingerprint: SHA256:ab:cd:ef:12:...
```

⚠️ **Back up `.sentinel/keys/` immediately.** If you lose these, you lose access to your encrypted commit history.

---

## Daily Workflow Commands

### `sentinel commit -m "message"`

Hash, encrypt, and commit your code. This replaces `git commit` in your workflow.

```bash
sentinel commit -m "your commit message"
```

**Flags:**
```
-m, --message string    Commit message (required)
    --proof-only        Hash and anchor to Bitcoin WITHOUT encrypting
                        Use this for open source projects
```

**What it does (in order):**
1. Checks your encryption keys exist
2. Loads the AES-256 key
3. Runs `git add .` — auto-stages all changes
4. Checks that there are staged changes to commit
5. Collects all tracked source files (skips binaries, `.sentinel/`, markdown)
6. SHA-256 hashes every file — saves hash record to `.sentinel/hashes/`
7. Registers proof record to disk synchronously (status: pending)
8. AES-256-GCM encrypts every source file
9. Runs `git add .` again — re-stages the encrypted versions
10. Runs `git commit -m "message"`
11. Decrypts all files back — you keep working normally
12. Submits root hash to OpenTimestamps → Bitcoin (async, background, free)

**Examples:**
```bash
# Full protection — encrypts code before pushing
sentinel commit -m "add authentication middleware"

# Proof-only — code stays readable, authorship still proven on Bitcoin
# Perfect for open source projects like Sentinel itself
sentinel commit -m "release v1.0" --proof-only
# → Checking encryption keys... found.
# → Loading AES-256 key... done.
# → Staging all changes (git add .)... done.
# → Checking for changes to commit... changes found.
# → Collecting tracked files... 23 files found.
# → Hashing plaintext files (SHA-256)... done.
#    Root hash:  acc5c3c3d2794bda5d57c82a2961e459...
#    Hash file:  .sentinel/hashes/1773907519.json
# → Encrypting files (AES-256-GCM)... done.
# → Staging encrypted files... done.
# → Running git commit... done.
# → Restoring plaintext locally... done.
# → Anchoring to Bitcoin (OpenTimestamps, async)...
# ✓ Commit protected successfully!
```

---

### `sentinel push`

Push encrypted code to your remote repository.

```bash
sentinel push
```

**What it does:**
- Verifies Sentinel protection status
- Runs `git push`
- Confirms remote only contains encrypted code

**Example:**
```bash
sentinel push
# → Pushing to remote (encrypted)... done.
# ✓ Pushed. Remote only contains encrypted code.
```

---

### `sentinel pull`

Pull from remote and auto-decrypt locally.

```bash
sentinel pull
```

**What it does:**
- Runs `git pull`
- Loads your local AES key
- Decrypts all source files automatically

**Example:**
```bash
sentinel pull
# → Pulling from remote... done.
# → Loading AES-256 key... done.
# → Decrypting files locally... done.
# ✓ Pull complete. Files decrypted locally.
```

---

### `sentinel status`

Show Sentinel protection status and git status.

```bash
sentinel status
```

**Example output:**
```
  Protection Layers:
    ✓  PREVENT   AES-256 encryption active
    ✓  PROVE     SHA-256 hashing active
    ✓  DETECT    AI interrogation configured
    ✓  ACCESS    2 active collaborators

  Git Status:
  On branch main
  nothing to commit, working tree clean
```

---

### `sentinel log`

Show Sentinel-annotated commit history.

```bash
sentinel log
```

Wraps `git log --oneline --graph --decorate -20` with Sentinel header.

---

## Proof Commands (Phase 3)

### `sentinel proof` / `sentinel proof status`

Show the status of your latest Bitcoin proof.

```bash
sentinel proof
sentinel proof status
```

**Example output:**
```
  Root Hash:
    1f79865af908e63974c41c060eaf3695b2fe86720230817a52dac039284e55e3

  Status:      ⏳ PENDING — awaiting Bitcoin confirmation (up to 24hrs)
  Submitted:   Thu, 19 Mar 2026 08:12:06 UTC
  .ots file:   .sentinel/proofs/1f79865af908e639.ots
  Calendar:    https://a.pool.opentimestamps.org
```

---

### `sentinel proof list`

List all proof records for this repository.

```bash
sentinel proof list
```

**Example output:**
```
  #   HASH                   STATUS          SUBMITTED
  -   ----                   ------          ---------
  1   acc5c3c3d2794bda...    ✓ confirmed     2026-03-19 07:34:35
  2   1f79865af908e639...    ⏳ pending       2026-03-19 08:12:06

  Total: 2 proof(s)
```

---

### `sentinel proof upgrade`

Check if pending proofs have been confirmed on Bitcoin.

```bash
sentinel proof upgrade
```

Bitcoin blocks are mined approximately every 10 minutes. Full confirmation (6 blocks) takes about 1 hour. Run this command after waiting to upgrade `pending` → `confirmed`.

**Example output:**
```
  → Checking 1f79865af908e639... CONFIRMED on Bitcoin! ✓

  Checked: 1 pending  |  Upgraded: 1
```

---

### `sentinel proof verify <hash>`

Verify that a specific root hash has a valid proof record.

```bash
sentinel proof verify <root-hash>
```

**Arguments:**
```
<root-hash>    The full SHA-256 root hash to verify
```

**Example:**
```bash
sentinel proof verify 1f79865af908e63974c41c060eaf3695b2fe86720230817a52dac039284e55e3

# ✓ Proof found!
#   Root Hash:   1f79865af908e63974c41c060eaf3695...
#   Submitted:   Thu, 19 Mar 2026 08:12:06 UTC
#   Status:      ✓ CONFIRMED on Bitcoin
#   .ots SHA-256: abc123...
#   .ots file integrity: OK
```

---

## AI Interrogation Commands (Phase 4)

### `sentinel scan models`

List available models for each supported AI provider.

```bash
sentinel scan models
```

**Example output:**
```
  OpenAI
    ✓ gpt-4o                              (default)
      gpt-4-turbo
      gpt-3.5-turbo

  Anthropic
    ✓ claude-haiku-4-5-20251001           (default)
      claude-opus-4-6
      claude-sonnet-4-6

  Google Gemini (free)
    ✓ gemini-2.0-flash                    (default)
      gemini-1.5-pro
      gemini-2.0-pro-exp

  Ollama (local) (free)
    ✓ codellama                           (default)
      llama3
      deepseek-coder
      mistral
```

---

### `sentinel scan config`

Configure your AI provider and API key.

```bash
sentinel scan config --provider <provider> --key <api-key> [--model <model>]
```

**Flags:**
```
-p, --provider string    AI provider: openai, anthropic, gemini, ollama (required)
-k, --key string         Your API key (not required for ollama)
-m, --model string       Model name override (optional — uses provider default if omitted)
```

**Examples:**
```bash
# OpenAI
sentinel scan config --provider openai --key sk-...

# Anthropic
sentinel scan config --provider anthropic --key sk-ant-...

# Google Gemini (has a free tier)
sentinel scan config --provider gemini --key AIza... --model gemini-2.0-flash

# Ollama — completely free, runs locally
sentinel scan config --provider ollama

# Override model
sentinel scan config --provider openai --key sk-... --model gpt-4-turbo
```

**Environment variable alternative:**
```bash
export SENTINEL_AI_PROVIDER=gemini
export SENTINEL_AI_KEY=AIza...
export SENTINEL_AI_MODEL=gemini-2.0-flash
```

---

### `sentinel scan run`

Run AI interrogation against your codebase.

```bash
sentinel scan run [--files file1.go,file2.go]
```

**Flags:**
```
-f, --files strings    Specific files to scan (default: all tracked source files)
```

**What it does:**
1. Loads your AI provider config
2. Collects all tracked Go source files
3. Parses every file using Go's AST parser
4. Generates probe prompts for each significant function
5. Sends probes to your configured AI provider
6. Compares AI output to your original using token + structural similarity
7. Classifies evidence weight per function
8. Saves full report to `.sentinel/scan_<timestamp>.json`

**Example:**
```bash
sentinel scan run
# → Loading AI provider config... using Google Gemini (gemini-2.0-flash)
# → Collecting source files... 55 files.
# → Analysing code and generating probes... 198 probes generated.
#
#   Interrogating Google Gemini with 198 probes...
#
#   [1/198] Probing: ValidateJwtToken  → 61% similarity [moderate evidence]
#   [2/198] Probing: GenerateJwtToken  → 66% similarity [moderate evidence]
#   [3/198] Probing: NewClient         → 21% similarity [no evidence]
#   ...
#
#   ─────────────────────────────────
#   SCAN COMPLETE — Evidence Summary
#   ─────────────────────────────────
#
#   Strong evidence:    0 function(s)
#   Moderate evidence:  2 function(s)
#   Weak evidence:      8 function(s)
#   No evidence:        188 function(s)
```

**Scan specific files:**
```bash
sentinel scan run --files internal/auth/jwt.go,internal/http/middleware.go
```

---

### `sentinel scan report`

View the latest scan report.

```bash
sentinel scan report
```

**Example output:**
```
  SENTINEL SCAN REPORT

  Date:     Thu, 19 Mar 2026 09:15:00 UTC
  Provider: Google Gemini (gemini-2.0-flash)
  Files:    55 scanned, 198 probes run

  ── MODERATE EVIDENCE (2) ──

    func ValidateJwtToken
    File:       internal/auth/jwt.go
    Similarity: 61%  (structural: 65%, tokens: 54%)
    Uniqueness: 68/100

    func GenerateJwtToken
    File:       internal/auth/jwt.go
    Similarity: 66%  (structural: 70%, tokens: 59%)
    Uniqueness: 71/100

  ── NO EVIDENCE (196 functions) ──

  Report file: .sentinel/scan_1773910500.json
```

---

## Access Control Commands (Phase 5)

### `sentinel grant <username>`

Derive and output a unique decryption key for a collaborator.

```bash
sentinel grant <username>
```

**Arguments:**
```
<username>    The collaborator's identifier (e.g. GitHub username)
```

**What it does:**
- Derives a unique AES-256 key for this collaborator using HKDF-SHA256
- Saves derived key to `.sentinel/keys/collaborators/<username>.key`
- Logs the grant in `.sentinel/collaborators.json`
- Outputs a shareable key string to copy and send

**Example:**
```bash
sentinel grant alice

# → Loading master key... found.
# → Deriving key for 'alice' (HKDF-SHA256)... done.
# ✓ Access granted!
#
#   ─────────────────────────────────────────────────────────
#   SHAREABLE KEY — Send this to your collaborator SECURELY:
#   ─────────────────────────────────────────────────────────
#
#   sentinel:YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXo...==:alice
#
#   ─────────────────────────────────────────────────────────
#
#   ⚠  Send via Signal, encrypted email, or in person.
#      NEVER send via Slack, plain email, or GitHub.
```

---

### `sentinel revoke <username>`

Revoke a collaborator's access.

```bash
sentinel revoke <username>
sentinel revoke <username> --rotate
```

**Flags:**
```
-r, --rotate    Hard revoke: generate new master key (cryptographically certain)
```

**Soft revoke (default):**
```bash
sentinel revoke alice
# → Marks alice as revoked in registry
# → Deletes local derived key file
# ⚠  Their existing key copy still works until hard revoke
```

**Hard revoke (--rotate):**
```bash
sentinel revoke alice --rotate
# → Marks alice as revoked
# → Generates completely NEW master key
# → Re-derives keys for all remaining active collaborators
# → Alice's key is now permanently invalid
# → Outputs new keys for active collaborators to re-share
```

Use `--rotate` when: someone leaves under difficult circumstances, you suspect a key was compromised, or you need cryptographic certainty rather than just administrative revocation.

---

### `sentinel whohas`

List everyone with access — active and revoked.

```bash
sentinel whohas
```

**Example output:**
```
  USERNAME      STATUS       GRANTED AT          KEY HASH
  --------      ------       ----------          --------
  alice         ● active     2026-03-19 10:00    a3f8c2d1
  bob           ● active     2026-03-19 10:05    9e4b7f12
  carol         ✗ revoked    2026-01-15 09:00    7f3a1c84

  Active: 2  |  Total: 3
```

---

### `sentinel collab join --key <key>`

Install a collaborator key received from the repo owner. Run this after the owner sends you a key via `sentinel grant`.

```bash
sentinel collab join --key "sentinel:base64key...:username"
```

**Flags:**
```
-k, --key string    The key string shared by the repo owner (required)
```

**Example:**
```bash
sentinel collab join --key "sentinel:YWJjZGVmZ2hpamtsbW5vcHFy...==:alice"

# → Parsing key... done.
# → Verifying key can decrypt... done.
# ✓ Key installed successfully!
#
#   This key was issued for: alice
#
#   You can now:
#     git clone <repo>
#     sentinel pull
#     sentinel commit -m ...
```

---

### `sentinel collab status`

Check whether a collaborator key is installed on this machine.

```bash
sentinel collab status
```

**Example output (key installed):**
```
  ✓ Decryption key installed — you have access.
  Run 'sentinel pull' to pull and decrypt the latest code.
```

**Example output (no key):**
```
  ✗ No decryption key found.
  Ask the repo owner to run: sentinel grant <your-username>
  Then run: sentinel collab join --key <the-key>
```

---

## Full Command Tree

```
sentinel
├── init
├── keygen
├── commit        -m "message"
├── push
├── pull
├── status
├── log
│
├── proof
│   ├── status
│   ├── list
│   ├── upgrade
│   └── verify    <hash>
│
├── scan
│   ├── models
│   ├── config    --provider --key --model
│   ├── run       --files
│   └── report
│
├── grant         <username>
├── revoke        <username>  [--rotate]
├── whohas
│
└── collab
    ├── join      --key
    └── status
```

---

## Environment Variables

```bash
SENTINEL_AI_PROVIDER    AI provider name (openai, anthropic, gemini, ollama)
SENTINEL_AI_KEY         API key for the configured provider
SENTINEL_AI_MODEL       Model name override
```

Environment variables always take priority over `.sentinel/ai_config.json`.

---

## File Exclusions

Sentinel automatically skips these file types during encryption:

```
Compiled binaries     files with no extension + executable bit
Markdown files        *.md
Text files            *.txt
Config files          sentinel.toml, .gitignore, go.sum, Makefile, LICENSE
.sentinel/ directory  all internal Sentinel files
```

Source files encrypted by default:

```
.go  .js  .ts  .py  .rs  .c  .cpp  .h  .java  .rb  .php
.swift  .kt  .cs  .json  .yaml  .yml  .toml  .xml  .html
.css  .sh  .env
```

---

*For the full technical explanation of how each command works internally, see [HOW_SENTINEL_WORKS.md](HOW_SENTINEL_WORKS.md).*

---

## Sentinel vs Git — When to Use Which

This is important. **If you use `git commit` or `git push` directly, Sentinel's encryption is bypassed.** Your plaintext code reaches GitHub.

```
ALWAYS USE SENTINEL FOR:         USE GIT DIRECTLY FOR:
  sentinel commit ✅               git branch       ✅
  sentinel push   ✅               git merge        ✅
  sentinel pull   ✅               git stash        ✅
  sentinel status ✅               git diff         ✅
  sentinel log    ✅               git clone        ✅
                                   git remote       ✅
                                   git add          ✅ (sentinel does this for you)
                                   git rebase       ✅
                                   git cherry-pick  ✅
```

**The rule:** Anything that moves code to or from a remote — use Sentinel. Anything that is purely local repo management — git is fine.

Why: `sentinel commit` is the encryption step. `sentinel push` verifies that only encrypted blobs are going out. Bypassing either with raw git means your code travels in plaintext.

---

## Installation

### One-Line Install (recommended)

```bash
curl -sSL https://raw.githubusercontent.com/ttomsin/sentinel/main/install.sh | bash
```

This automatically checks Go version, clones the repo, builds the binary, and installs to `/usr/local/bin/sentinel`.

### Manual Install

```bash
git clone https://github.com/ttomsin/sentinel.git
cd sentinel
go build -o sentinel
sudo mv sentinel /usr/local/bin/sentinel
```

**Requirements:** Go 1.22+, Git
