# How Sentinel Works — Complete Technical Reference

This document covers all five phases of Sentinel from top to bottom. Every cryptographic choice, every design decision, every command explained.

---

## Table of Contents

1. [The Big Picture](#1-the-big-picture)
2. [Phase 1 — CLI Foundation](#2-phase-1--cli-foundation)
3. [Phase 2 — Encryption (PREVENT)](#3-phase-2--encryption-prevent)
4. [Phase 3 — Blockchain (PROVE)](#4-phase-3--blockchain-prove)
5. [Phase 4 — AI Interrogation (DETECT)](#5-phase-4--ai-interrogation-detect)
6. [Phase 5 — Access Control (ACCESS)](#6-phase-5--access-control-access)
7. [The .sentinel Directory](#7-the-sentinel-directory)
8. [The Core Commit Pipeline](#8-the-core-commit-pipeline)
9. [Key Management](#9-key-management)
10. [Do You Need to Store Anything Manually?](#10-do-you-need-to-store-anything-manually)

---

## 1. The Big Picture

```
WITHOUT SENTINEL:
Your code on GitHub → AI crawler reads it → AI trains on it → competes with you
You have no proof, no recourse, no protection.

WITH SENTINEL:
Your code on GitHub → AI crawler reads encrypted noise → cannot train       ✓
Root hash on Bitcoin → you have timestamped proof of authorship             ✓
AI interrogation → you can detect if a model trained on your work           ✓
HKDF access control → only authorised people can decrypt                    ✓
```

Five phases. Each solves a different part of the problem.

---

## 2. Phase 1 — CLI Foundation

Sentinel is built on Cobra — the same CLI framework used by Kubernetes, Docker, and Hugo. Every command follows the same pattern as Git so developers feel at home immediately.

```bash
sentinel init       # like git init
sentinel commit     # like git commit (but with protection)
sentinel push       # like git push (but verified encrypted)
sentinel pull       # like git pull (but auto-decrypts)
```

Internally, Sentinel wraps git via `os/exec` — every git operation goes through `git/wrapper.go`. This clean separation means if git changes behaviour, only one file needs updating.

`sentinel init` creates the `.sentinel/` directory structure and adds keys to `.gitignore`. This is non-negotiable — keys must never reach a remote.

---

## 3. Phase 2 — Encryption (PREVENT)

### SHA-256 Hashing

Before any encryption, Sentinel hashes every source file:

```go
data, _ := os.ReadFile(path)
hash := sha256.Sum256(data)  // [32]byte — 256-bit fingerprint
```

All per-file hashes are combined into a single **root hash**:

```go
combined := ""
for _, h := range hashes {
    combined += h.Hash
}
rootHash := sha256.Sum256([]byte(combined))
```

This root hash fingerprints your entire codebase at that moment. Change one character anywhere — the root hash changes completely. This is what gets anchored to Bitcoin.

### AES-256-GCM Encryption

AES (Advanced Encryption Standard) with GCM (Galois/Counter Mode) provides both encryption and authentication — meaning tampered files are detected on decryption.

```go
block, _ := aes.NewCipher(key)      // 256-bit key
gcm, _ := cipher.NewGCM(block)

// Random nonce per file — CRITICAL: never reuse a nonce
nonce := make([]byte, gcm.NonceSize())
io.ReadFull(rand.Reader, nonce)

// Encrypt: prepend nonce to ciphertext
ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
// result = [12-byte nonce][encrypted content][16-byte auth tag]
```

The nonce is random per-file per-commit — the same file encrypted twice looks completely different. This prevents even a pattern-matching attack.

### The Critical Re-Staging Step

This is the subtlest part of the whole system. Git's staging area is a snapshot — it doesn't automatically update when you change files on disk.

```
WRONG order:                    CORRECT order:
git add .    ← stage plaintext  git add .    ← stage plaintext
encrypt      ← disk encrypted   encrypt      ← disk encrypted
git commit   ← commits          git add .    ← RE-STAGE encrypted ← KEY
             PLAINTEXT ❌        git commit   ← commits CIPHERTEXT ✓
```

Without the second `git add .`, GitHub would receive your plaintext code even though it looks like Sentinel encrypted it. This bug existed in early versions and was caught during testing.

---

## 4. Phase 3 — Blockchain (PROVE)

### Why Bitcoin

Bitcoin's blockchain is:
- **Immutable** — written records cannot be altered
- **Decentralised** — no company controls it, including Sentinel
- **Public** — anyone can verify independently
- **Permanent** — uninterrupted since 2009

A Bitcoin timestamp is the closest thing to a universal neutral clock.

### OpenTimestamps — Why It's Free

Writing directly to Bitcoin costs a gas fee. OpenTimestamps solves this by batching:

```
Your hash ─────────────────────┐
Other hashes ──────────────────┤── Merkle tree → ONE Bitcoin tx (they pay)
Thousands more ────────────────┘
```

Your hash is aggregated into a Merkle tree with thousands of others. One transaction goes to Bitcoin. Each individual hash can prove it was part of that root. Cost to you: zero.

### Proof Status Flow

```
sentinel commit
      ↓
RegisterProof() — saves record to disk SYNCHRONOUSLY (status: pending)
      ↓ (goroutine)
AnchorHash() — HTTP POST to OTS calendar servers
      ↓
.ots file saved to .sentinel/proofs/
      ↓ (~1 hour)
sentinel proof upgrade
      ↓
Fetches upgraded .ots from calendar server
Status: pending → confirmed ✓
```

The proof record is saved synchronously before the HTTP call — so `sentinel proof status` works immediately after commit, even if the process exits before the async call completes. This was a bug in early versions.

---

## 5. Phase 4 — AI Interrogation (DETECT)

The core insight: there is a difference between an AI knowing that a problem exists and knowing the specific way a particular developer solved it.

### The Probe Pipeline

```
Source files → AST parser (go/ast) → extract function signatures
      ↓
For each function: generate a natural language probe prompt
"Write a function that [does what this does] — without showing our code"
      ↓
Send probe to configured AI provider
      ↓
Compare AI response to original using two layers:
  Layer 1: Token similarity (35% weight) — surface matches
  Layer 2: AST structural similarity (65% weight) — logic shape
      ↓
Evidence classification: strong / moderate / weak / none
```

### Why AST Structural Similarity Matters More

Token similarity can be evaded — rename variables, reformat, done. AST structural similarity is much harder to evade because it captures the logical shape of code with all names normalised:

```
Your code AST:      DECLARE CALL:ParseWithClaims IF CALL:Errorf RETURN RANGE
AI output AST:      DECLARE CALL:ParseWithClaims IF CALL:Errorf RETURN RANGE
                    ↑ identical logical structure despite different variable names
```

### Evidence Weights

```
≥75% combined score → STRONG EVIDENCE
≥50% combined score → MODERATE EVIDENCE
≥25% combined score → WEAK EVIDENCE
<25% combined score → NO EVIDENCE
```

Real test result: `ValidateJwtToken → 66% [moderate]`. The developer confirmed the code was originally AI-assisted. Phase 4 detected the training fingerprint.

---

## 6. Phase 5 — Access Control (ACCESS)

### The Core Problem

If every collaborator uses the master key, you can never revoke anyone without invalidating everyone. You need per-person keys that all decrypt the same data.

### HKDF Key Derivation

HKDF (HMAC-based Key Derivation Function, RFC 5869) solves this:

```go
info := []byte("sentinel-collab-v1:" + username)
hkdfReader := hkdf.New(sha256.New, masterKey, salt, info)
derived := make([]byte, 32)
io.ReadFull(hkdfReader, derived)
```

Properties:
- Same inputs → same derived key (deterministic)
- Alice's key ≠ Bob's key (username binds each key)
- Cannot reverse-engineer master key from derived key
- Per-repo salt prevents cross-repo key reuse

### Soft vs Hard Revoke

```
Soft revoke (sentinel revoke alice):
  → Marks Alice as revoked in registry
  → Deletes her local key file
  → Her existing copy still works

Hard revoke (sentinel revoke alice --rotate):
  → Generates NEW master key
  → Re-derives keys for all remaining active collaborators
  → Alice's key permanently invalid even if she kept a copy
  → Remaining collaborators need new keys re-shared
```

Use soft revoke for departing contributors. Use hard revoke when security is critical — e.g. terminated employee, suspected key compromise.

### The Shareable Key Format

```
sentinel:base64encodedkey==:username
```

This format is:
- Self-describing (prefix identifies it as a sentinel key)
- Compact (base64 encoding)
- Attribution-preserving (username included for audit)
- Safe to copy-paste without corruption

---

## 7. The .sentinel Directory

```
.sentinel/
├── keys/                          ← NEVER committed (.gitignore)
│   ├── master.key                 ← AES-256 key (hex, 64 chars)
│   ├── master.priv                ← Ed25519 private key (hex)
│   ├── master.pub                 ← Ed25519 public key (hex, safe to share)
│   ├── repo.salt                  ← 32-byte random HKDF salt
│   └── collaborators/
│       ├── alice.key              ← Alice's derived key (hex)
│       └── bob.key                ← Bob's derived key (hex)
├── hashes/                        ← Committed to git
│   ├── 1773905675.json            ← Per-commit hash record
│   └── 1773907519.json
├── proofs/                        ← Committed to git
│   ├── index.json                 ← All proof records
│   └── 1f79865af908e639.ots       ← Bitcoin proof file
├── scan_<timestamp>.json          ← AI interrogation reports
├── collaborators.json             ← Access registry
└── ai_config.json                 ← AI provider config (0600 permissions)
```

---

## 8. The Core Commit Pipeline

### Full Protection Mode — `sentinel commit -m "message"`

```
1.  Check keys exist         → error if no master.key found
2.  Load AES-256 key         → read .sentinel/keys/master.key
3.  git add .                → stage all changes
4.  Check staged changes     → stop early if nothing to commit
5.  Collect source files     → git ls-files (minus binaries, .sentinel/)
6.  SHA-256 hash each file   → PROVE layer
7.  Compute root hash        → single fingerprint of entire codebase
8.  RegisterProof()          → save proof record to disk SYNCHRONOUSLY
9.  AES-256-GCM encrypt      → PREVENT layer (files become noise)
10. git add . (again)        → RE-STAGE the encrypted versions ← critical
11. git commit -m "message"  → Git commits encrypted blobs
12. AES-256-GCM decrypt      → restore your working files
13. AnchorHash() goroutine   → async HTTP to OpenTimestamps → Bitcoin
```

Steps 9 and 10 together are the most important: encrypt on disk, then re-stage so Git commits the ciphertext not the plaintext.

### Proof-Only Mode — `sentinel commit -m "message" --proof-only`

For open source projects that want authorship proof WITHOUT encrypting the code. Sentinel itself uses this mode.

```
1.  git add .                → stage all changes
2.  Check staged changes     → stop early if nothing to commit
3.  Collect source files     → git ls-files
4.  SHA-256 hash each file   → PROVE layer (same as full mode)
5.  Compute root hash        → same as full mode
6.  RegisterProof()          → same as full mode
7.  [NO ENCRYPTION]          → code stays plaintext
8.  git commit -m "message"  → Git commits readable plaintext
9.  AnchorHash() goroutine   → async HTTP to OpenTimestamps → Bitcoin
```

Everything is identical except steps 9-12 of full mode (encrypt, re-stage, decrypt) are skipped entirely. The blockchain proof is just as strong — your root hash is still on Bitcoin. The code is just publicly readable, which is the whole point for open source.

**When to use --proof-only:**
- Your project is intentionally open source (like Sentinel itself)
- You want authorship proof without hiding the code
- You're a library author who wants to prove priority of invention
- You want to timestamp a specification or design document

---

## 9. Key Management

| File | Type | Permission | Purpose |
|------|------|-----------|---------|
| `master.key` | AES-256 (32 bytes) | 0600 | Encrypts/decrypts source files |
| `master.priv` | Ed25519 private | 0600 | Signs proof certificates |
| `master.pub` | Ed25519 public | 0644 | Verifies your signatures |
| `repo.salt` | Random 32 bytes | 0600 | HKDF salt for key derivation |

All keys are stored as hex strings. All sensitive files are `0600` — owner read/write only. The `.sentinel/keys/` directory is in `.gitignore` — it is physically prevented from being committed.

**Back up your keys.** A password manager, encrypted USB, or secure cloud vault. Losing `master.key` means losing access to your entire encrypted commit history.

---

## 10. Do You Need to Store Anything Manually?

**No.** Everything is automatic.

| What you need | Where it lives | How to access it |
|---------------|----------------|-----------------|
| Latest root hash | `.sentinel/proofs/index.json` | `sentinel proof status` |
| All root hashes | `.sentinel/proofs/index.json` | `sentinel proof list` |
| Bitcoin proof | `.sentinel/proofs/*.ots` | `sentinel proof status` |
| Per-file hashes | `.sentinel/hashes/*.json` | open JSON directly |
| AI scan results | `.sentinel/scan_*.json` | `sentinel scan report` |
| Collaborator access | `.sentinel/collaborators.json` | `sentinel whohas` |

The only manual action required: **back up `.sentinel/keys/`**. Everything else travels with the repository automatically.

---

*Sentinel is open source. The protocol is transparent. Trust the math.*
