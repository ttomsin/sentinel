# Why Phase 5 Matters — Access Control as Protection

Without access control, encryption is only half a solution. Phase 5 completes the protection story — making Sentinel viable for teams, enterprises, and any developer who wants to share code without losing control of it.

---

## Encryption Without Access Control Is a Dead End

Imagine Sentinel had only Phases 1-4. Your code is encrypted. Your proof is on Bitcoin. Your AI interrogation is running.

Now your company hires a contractor. They need to work on the codebase. What do you do?

```
Option A: Share your master key.
  Problem: When the contract ends, they still have your master key forever.
  
Option B: Don't use Sentinel for collaborative work.
  Problem: Sentinel becomes useless for anything beyond solo projects.
  
Option C: Phase 5 — derive a unique key per person.
  Result: Contractors get access. Contracts end. Access is revoked.
          Your master key never leaves your hands.
```

Without Phase 5, Sentinel would be a solo developer tool. With Phase 5, it works for any team.

---

## The Real-World Scenarios Phase 5 Handles

### Scenario 1 — The Contractor

You hire a freelancer to work on a feature for three months.

```
Day 1:    sentinel grant contractor-jane
Day 90:   sentinel revoke contractor-jane
```

Jane had full access for the contract period. When it ended, her key was invalidated. She has no path to your codebase going forward. She cannot share your key with anyone, because the key is uniquely derived for her — anyone receiving it would know it's traced back to Jane.

### Scenario 2 — The Terminated Employee

An employee leaves under difficult circumstances. You need cryptographic certainty that they cannot access the codebase.

```
sentinel revoke former-employee --rotate
```

The `--rotate` flag generates a completely new master key. The former employee's derived key now decrypts nothing — not current files, not future commits. Even if they saved their key string, it is mathematically useless.

Yes, this requires re-sharing keys with your remaining team. That's a small price for certainty.

### Scenario 3 — The Open Source Maintainer

You maintain an open source project but keep the proprietary implementation private. You want trusted contributors to be able to decrypt the code while keeping AI scrapers out.

```
sentinel grant contributor-a
sentinel grant contributor-b
sentinel grant contributor-c
```

Each contributor gets a unique key. If contributor-b goes rogue and leaks their key — you know exactly who leaked it (key hash in the audit log). You hard revoke contributor-b. Contributor-a and contributor-c are unaffected.

### Scenario 4 — The Acquisition

Your startup is acquired. Due diligence requires the acquiring company's technical team to review your codebase.

```
sentinel grant due-diligence-team
# Review happens
sentinel revoke due-diligence-team
```

If the deal falls through, their access ends immediately. If the deal closes, they get added permanently. Your codebase was protected throughout the process.

---

## Why This Matters Specifically for AI Protection

Phase 5 isn't just about team management. It connects directly to the core mission of Sentinel — protecting code from AI training.

### It Closes the Human Leak

The most sophisticated encryption in the world doesn't help if a human with access can exfiltrate your code. Phase 5 controls who the humans are.

```
Without Phase 5:
  You encrypt your code ✓
  You share master key with 5 colleagues
  One colleague pastes code into ChatGPT "just to ask a question"
  That code is now potentially in a training dataset
  You have no way to know who did it

With Phase 5:
  Each colleague has a unique derived key
  If code leaks, the key hash in the audit log narrows it down
  Hard revoke + rotate contains future damage
  Accountability exists
```

### It Makes Revocation Meaningful

If you discover an AI company somehow obtained your code, the question becomes: how? Phase 5 gives you an audit trail to investigate. When did access change? Who had keys during the relevant window? The registry answers these questions.

### It Supports the Legal Argument

When building a case that an AI company trained on your code, the question of access is relevant. "Only five people had keys to this codebase during the period in question. None of them are affiliated with the AI company. The code was encrypted on GitHub. How did the model learn it?"

That's a stronger position than "my code was on GitHub and you could have scraped it."

---

## Why HKDF Was the Right Choice

Several alternatives were considered for Phase 5.

### Alternative 1 — Symmetric Key Per Person

Give each collaborator a completely independent AES key and re-encrypt all files for each person.

```
Problem: To add 10 collaborators, encrypt every file 10 times.
         To revoke one, re-encrypt everything.
         Storage and performance cost scales with team size.
         Does not work for large codebases.
```

### Alternative 2 — Asymmetric Encryption Per Person

Use each collaborator's public key to encrypt the master key, share the encrypted master key.

```
Problem: Requires collecting everyone's public keys before they join.
         Infrastructure complexity (key servers, key distribution).
         Revoking still requires re-encrypting the master key copy.
         Overkill for the problem at hand.
```

### Alternative 3 — HKDF Derivation (chosen)

Derive unique keys from the master key using a one-way function.

```
Advantages:
  ✓ No extra storage — derived keys computed on demand
  ✓ Revoking one person doesn't affect others
  ✓ Master key never shared — only derived keys leave the owner
  ✓ Audit trail without storing actual keys
  ✓ Cross-repo isolation via salt
  ✓ Standard cryptography (RFC 5869) — well-reviewed, no custom crypto
```

HKDF is the same primitive used in TLS 1.3, Signal Protocol, and WhatsApp's end-to-end encryption. It was designed exactly for this use case.

---

## The Access Control and Blockchain Connection

Phase 3 and Phase 5 work together in a way that strengthens both.

Phase 3 timestamps your codebase on Bitcoin. Phase 5 controls who can read that codebase.

```
Timeline:
  March 2026:  Code committed, hash anchored to Bitcoin
  March 2026:  Codebase encrypted, only 3 people have keys
  April 2026:  AI company releases new model
  May 2026:    Phase 4 scan shows 78% similarity to your code

Question: How did the AI get your code?
  - GitHub shows only encrypted noise → not from scraping
  - Only 3 keys were ever issued → investigation narrows to 3 people
  - Collaborators.json shows access history → audit trail exists
  - Bitcoin proof shows code existed before model training → timing proven
```

The combination is significantly stronger than any single piece of evidence.

---

## What Phase 5 Does Not Solve

Honesty matters. Phase 5 is powerful but not a complete solution to every access problem.

**It does not prevent screen capture.** If someone with a valid key looks at the decrypted code on their screen and photographs it — there's no technical defence. This is true of every access control system.

**Soft revoke doesn't invalidate existing key copies.** A collaborator who saved their key string can still decrypt files committed before the hard revoke. For true certainty, `--rotate` is required.

**It doesn't prevent authorised users from leaking.** Phase 5 controls access and creates audit trails. It cannot stop a determined insider from extracting code. The audit trail helps identify the source after the fact.

These are known, accepted limitations. Phase 5 significantly raises the barrier and creates accountability. It does not claim to be perfect.

---

## The Principle Behind Phase 5

> Access should be granted intentionally, revoked decisively, and audited completely.

This is the same principle that governs enterprise security, government clearances, and responsible key management. Sentinel applies it to the specific problem of protecting developer code from AI training — making it accessible to individual developers and small teams without requiring a security team or infrastructure.

One command to grant. One command to revoke. A complete audit trail. That's Phase 5.