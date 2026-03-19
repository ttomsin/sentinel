# 🛡️ Sentinel

> **Your code. Your rights. Protected.**

Sentinel is a CLI tool written in Go that sits on top of Git and gives your source code five layers of protection against unauthorized AI training. It prevents AI scrapers from reading your code, proves you wrote it first on the Bitcoin blockchain, detects if AI models trained on your work, and controls who can decrypt your codebase.

```bash
sentinel commit -m "my feature"
```

One command. Your code is hashed, encrypted, committed, and anchored to Bitcoin — automatically.

---

## Why Sentinel Exists

AI companies train their models on billions of lines of code scraped from public repositories. Your years of work become their training data — without your consent, without compensation, and without any way to prove it happened.

Sentinel was built to change that. Not by asking AI companies for permission. By making protection automatic, evidence-based, and developer-owned.

---

## Five Phases of Protection

| Phase | Layer | What it does | Status |
|-------|-------|-------------|--------|
| 1 | **CLI Foundation** | Git wrapper — sentinel works like git | ✅ Complete |
| 2 | **PREVENT** | AES-256-GCM encryption before every push | ✅ Complete |
| 3 | **PROVE** | SHA-256 hashing + Bitcoin blockchain anchoring | ✅ Complete |
| 4 | **DETECT** | AI interrogation — finds if models trained on your code | ✅ Complete |
| 5 | **ACCESS** | HKDF key derivation — collaborator access control | ✅ Complete |

---

## Installation

### One-Line Install (recommended)

```bash
curl -sSL https://raw.githubusercontent.com/ttomsin/sentinel/main/install.sh | bash
```

Automatically checks your Go version, clones, builds, and installs to `/usr/local/bin`.

### Manual Install

```bash
git clone https://github.com/ttomsin/sentinel.git
cd sentinel
go build -o sentinel
sudo mv sentinel /usr/local/bin/sentinel
```

**Requirements:** Go 1.22+, Git

---

## Quick Start

```bash
cd your-project
sentinel init
sentinel keygen
sentinel commit -m "first protected commit"
sentinel push
sentinel proof status
```

---

## All Commands

### Setup
```bash
sentinel init                    # Initialize Sentinel in a Git repo
sentinel keygen                  # Generate master AES-256 + Ed25519 key pair
```

### Daily Workflow
```bash
sentinel commit -m "message"     # Hash → Encrypt → Git commit → Anchor to Bitcoin
sentinel push                    # Push encrypted code to remote
sentinel pull                    # Pull + auto-decrypt locally
sentinel status                  # Show protection layers + git status
sentinel log                     # Sentinel-annotated commit history
```

### Proof of Authorship (Phase 3)
```bash
sentinel proof                   # Show latest proof status
sentinel proof list              # All proofs for this repo
sentinel proof upgrade           # Check if Bitcoin confirmed pending proofs
sentinel proof verify <hash>     # Verify a specific root hash
```

### AI Interrogation (Phase 4)
```bash
sentinel scan models                               # List available models
sentinel scan config --provider gemini --key AIza... --model gemini-2.0-flash
sentinel scan config --provider openai --key sk-...
sentinel scan config --provider anthropic --key sk-ant-...
sentinel scan config --provider ollama             # Free, runs locally
sentinel scan run                                  # Run full interrogation
sentinel scan run --files path/to/file.go          # Scan specific files
sentinel scan report                               # View latest report
```

### Access Control (Phase 5)
```bash
sentinel grant <username>                # Derive + output a collaborator key
sentinel revoke <username>               # Soft revoke access
sentinel revoke <username> --rotate      # Hard revoke (rotate master key)
sentinel whohas                          # Audit table of all access
sentinel collab join --key "sentinel:..." # Install key from repo owner
sentinel collab status                   # Check your access status
```

---

## Sentinel vs Git — Important

Always use Sentinel for anything that touches your remote. Use git directly for local management.

```
USE SENTINEL:                 USE GIT DIRECTLY (fine):
  sentinel commit ✅            git branch, merge, stash
  sentinel push   ✅            git diff, rebase, clone
  sentinel pull   ✅            git remote, cherry-pick
```

If you run `git commit` or `git push` directly — encryption is bypassed and plaintext reaches GitHub.

---

## How It Works (Summary)

```
sentinel commit -m "message"
      ↓
① SHA-256 hash every file          → proof of authorship
② AES-256-GCM encrypt every file   → AI scrapers see noise
③ git add . (re-stage encrypted)   → Git commits ciphertext
④ git commit                       → encrypted blobs in history
⑤ Decrypt files locally            → you keep working normally
⑥ Anchor root hash to Bitcoin      → free, async, permanent proof
```

For the full technical deep-dive see [HOW_SENTINEL_WORKS.md](HOW_SENTINEL_WORKS.md).

---

## AI Interrogation — Phase 4 in Action

Real results from a test run:

```
[1/198] Probing: ValidateJwtToken  → 61% similarity [moderate evidence]
[2/198] Probing: GenerateJwtToken  → 66% similarity [moderate evidence]
[3/198] Probing: NewClient         → 21% similarity [no evidence]
```

The JWT functions scored moderate evidence. The developer confirmed the code was originally AI-assisted — Phase 4 detected the fingerprint. For code you wrote uniquely yourself, high scores become legally significant evidence.

---

## Collaboration

```bash
sentinel grant alice
# → prints: sentinel:base64key...:alice
# Send to Alice via Signal/encrypted email

# Alice runs:
sentinel collab join --key "sentinel:base64key...:alice"
sentinel pull   # auto-decrypts
```

---

## Documentation

| Document | Description |
|----------|-------------|
| [HOW_SENTINEL_WORKS.md](HOW_SENTINEL_WORKS.md) | Complete technical deep-dive |
| [PHASE4_HOW_IT_WORKS.md](PHASE4_HOW_IT_WORKS.md) | AI interrogation walkthrough |
| [PHASE4_WHY_IT_MATTERS.md](PHASE4_WHY_IT_MATTERS.md) | Why detecting AI training matters |
| [PHASE5_HOW_IT_WORKS.md](PHASE5_HOW_IT_WORKS.md) | Collaborator access control |
| [PHASE5_WHY_IT_MATTERS.md](PHASE5_WHY_IT_MATTERS.md) | Why access control matters |
| [PROBLEMS_WE_NAVIGATED.md](PROBLEMS_WE_NAVIGATED.md) | Hard problems and how we solved them |
| [FUTURE.md](FUTURE.md) | Roadmap and future directions |

---

## Roadmap

- [x] Phase 1 — CLI skeleton + Git wrapper
- [x] Phase 2 — AES-256 encryption + SHA-256 hashing
- [x] Phase 3 — Bitcoin blockchain anchoring
- [x] Phase 4 — AI interrogation + similarity detection
- [x] Phase 5 — Collaborator access control
- [ ] Phase 6 — Multi-language support (JS, Python, Rust)
- [ ] Phase 7 — Sentinel Hub (cloud registry + team dashboard)
- [ ] Phase 8 — Legal report generator (court-ready PDF)

---

*Built to protect developers. The math protects you. Not us.*
