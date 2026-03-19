# Sentinel — Future Directions

Sentinel's five phases solve the core problem: prevent AI from training on your code, prove you wrote it, detect if they did anyway, and control who can read it. But the work isn't finished. This document outlines where Sentinel goes next — from near-term improvements to long-term possibilities.

---

## Phase 6 — Multi-Language Support

**Current state:** Phase 4's AI interrogation and the structural similarity analysis only work with Go source files. The AST parser used (`go/ast`) is Go-specific.

**The problem:** Most real-world codebases aren't Go-only. JavaScript, TypeScript, Python, Rust, Java, and C++ represent the majority of code at risk.

**The plan:**

Each language needs its own AST parser:

| Language | Parser approach |
|----------|----------------|
| JavaScript/TypeScript | esprima or tree-sitter via subprocess |
| Python | Python's `ast` module via subprocess |
| Rust | `syn` crate via subprocess |
| Java | JavaParser via subprocess |
| C/C++ | clang AST via subprocess |

The cleanest architecture is a **language adapter interface**:

```go
type LanguageAdapter interface {
    ParseFile(path string) ([]Function, error)
    ExtractSignature(fn Function) string
    ScoreUniqueness(fn Function) int
}
```

Each language implements this interface. The probe generator and similarity analyser work with `[]Function` — they don't need to know which language they're dealing with.

**Priority order:** JavaScript/TypeScript first (largest attack surface), then Python, then Rust.

---

## Phase 7 — Sentinel Hub

**Current state:** Everything in Sentinel is local. Your keys are local. Your proofs are local. Your scan results are local. This is intentional — local-first means no server to trust, no service to shut down.

But local-only has limits:

- Team key management becomes manual when the team is large
- Proof verification requires the `.ots` file to be present
- Scan results can't be shared across a team's machines
- No central view of all your protected repositories

**The vision:** A self-hostable hub that teams can run on their own infrastructure.

```
sentinel hub init        # Start a Sentinel Hub instance
sentinel hub connect     # Connect a local repo to the hub
sentinel hub dashboard   # Web UI showing all repos, proofs, scan results
```

Key design constraints:
- **Self-hostable first** — teams control their own data
- **Keys stay local** — the hub never sees private keys
- **Optional** — Sentinel must remain fully functional without a hub
- **Open source** — the hub is part of the Sentinel project, not a paid service

The hub stores proof records, scan reports, and collaborator registries. It does NOT store private keys or decryption capability. Even if the hub is compromised, encrypted code remains encrypted.

---

## Phase 8 — Legal Report Generator

**Current state:** Sentinel produces evidence — blockchain proofs, similarity scores, hash records, scan reports. But this evidence is spread across JSON files and CLI output. A lawyer or court needs a document, not a collection of files.

**The plan:** `sentinel legal-report` generates a formatted PDF containing:

```
SENTINEL EVIDENCE REPORT
Generated: 2026-03-19
Repository: github.com/you/your-project

SECTION 1 — PROOF OF AUTHORSHIP
  Root Hash: 1f79865af908e63974c41c060eaf3695...
  Bitcoin Confirmation: Block #836,241
  Timestamp: 2026-03-19T08:05:19Z
  Verification: https://blockstream.info/tx/...
  [QR code for independent verification]

SECTION 2 — ENCRYPTION HISTORY
  First encrypted commit: 2026-03-19
  Push records: [table of commits with timestamps]
  Remote: github.com/you/your-project (encrypted blobs visible)

SECTION 3 — AI INTERROGATION RESULTS
  Provider tested: Google Gemini (gemini-2.0-flash)
  Date of scan: 2026-03-19
  Functions with strong evidence: 2
  Functions with moderate evidence: 7

  STRONG EVIDENCE:
    func ValidateJwtToken
    Similarity: 78% (structural: 82%, tokens: 71%)
    Uniqueness: 74/100
    [side-by-side comparison]

SECTION 4 — ACCESS AUDIT
  Key holders during relevant period: [table]
  Revocations: [table]

APPENDIX — TECHNICAL METHODOLOGY
  [explanation of SHA-256, AES-256, OpenTimestamps, AST analysis]
  [suitable for non-technical legal readers]
```

This report is the final piece of the evidence chain — the translation layer between cryptographic proof and legal argument.

---

## Phase 9 — The Access Problem, Revisited

Earlier in Sentinel's development, the collaborator access problem was noted as unsolved at the UX level. The cryptography works (HKDF derivation, key rotation). But the user experience of key sharing is still manual:

1. Owner runs `sentinel grant alice`
2. Owner copies the key string
3. Owner sends it to Alice via Signal
4. Alice pastes it into `sentinel collab join`

This works. But it's fragile — key strings can be truncated, corrupted, or accidentally sent over insecure channels.

**Potential solutions being considered:**

**QR Code Exchange**
```
sentinel grant alice --qr
# Displays a QR code on the terminal
# Alice scans it with the Sentinel mobile app
# Key installed directly, never touches a chat app
```

**Time-Limited Key Links**
```
sentinel grant alice --link --expires 24h
# Generates a one-time HTTPS URL
# Alice visits the URL on her machine
# Key is downloaded and installed automatically
# Link expires after use or after 24 hours
```

**In-Person Key Ceremony**
```
# Both devices on the same local network
sentinel grant alice --local
# Alice's machine: sentinel collab receive
# Direct encrypted transfer, never touches internet
```

None of these are implemented yet. The right solution depends on real usage patterns — how teams actually share keys in practice. Building this requires user feedback from real teams.

---

## The Bigger Problem — Policy and Standards

Technical tools alone cannot solve the AI training problem. Sentinel provides evidence and protection — but evidence needs somewhere to go.

**What needs to exist:**

**An industry standard for opt-out registries.** Similar to `robots.txt` but for AI training. A file in every repo that AI companies are obligated to respect. Sentinel could automatically generate and maintain this file.

**A compensation framework.** If AI companies are going to profit from training on developer code, there should be a mechanism for developers to claim compensation. Similar to music royalties — not perfect, but functional. Sentinel's proof infrastructure could integrate with such a system.

**Legal precedent.** The cases being litigated right now will define what's allowed. Sentinel's evidence reports are designed to be useful in that legal process. As precedent develops, the reports will be refined to match what courts find persuasive.

**Regulatory disclosure requirements.** Several jurisdictions are considering requiring AI companies to disclose training data sources. Sentinel's blockchain-anchored proofs would be directly relevant to such a disclosure regime.

Sentinel can't create these policy frameworks alone. But it can make developers visible in the conversation — by giving them evidence, giving them protection, and giving them standing to demand accountability.

---

## What Will Not Change

Some things are permanent design decisions:

**Sentinel will always be a CLI tool.** The command line is where developers live. A GUI adds complexity without adding capability.

**Keys will always be local.** No server ever holds your private key. No cloud service is required. No account needed.

**The core will always be open source.** The cryptography is standard. The protocol is transparent. Trust must be verifiable — not assumed.

**OpenTimestamps will always be free.** If the free anchoring mechanism ever fails, the fallback is direct Bitcoin OP_RETURN anchoring. Developers will never be forced to pay for proof.

**Go will remain the implementation language.** Single binary. Cross-platform. Fast. The right tool for a security CLI.

---

## The End Goal

The end goal of Sentinel is to make it the default way developers manage code that they care about protecting. Not a special tool for security-conscious developers — a normal part of the workflow, like `.gitignore` or `README.md`.

```
mkdir my-project
cd my-project
git init
sentinel init     ← as natural as git init
sentinel keygen
sentinel commit -m "first commit"
```

When that's the default — when protection is automatic rather than exceptional — the conversation with AI companies changes. It becomes harder to claim that scraping public repos is acceptable when the majority of public repos are encrypted and their authors have Bitcoin-anchored proof of authorship.

That's the long game. Sentinel is one piece of it.