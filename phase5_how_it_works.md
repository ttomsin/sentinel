# How Phase 5 Works — Collaborator Access Control

Phase 5 solves a problem that every encrypted system eventually faces: **how do you let trusted people in, while keeping the ability to lock them out?**

This document explains the cryptography behind Sentinel's access control, how keys are derived, how sharing works, and what revocation actually means.

---

## The Problem to Solve

Sentinel encrypts your codebase with a master AES-256 key. Only someone with that key can decrypt the files. That's great for keeping AI scrapers out — but what about your team?

The naive solution — share your master key with everyone — has a fatal flaw:

```
You share master key with Alice, Bob, and Carol.
Carol leaves the company.
You want to revoke Carol's access.

But Carol still has the master key.
You can't un-share something that's been shared.
The only option is to generate a new master key
and re-share with Alice and Bob.
Now every time anyone leaves — the entire team is disrupted.
```

At scale, this is unworkable. Phase 5 solves it with **key derivation**.

---

## The Solution — HKDF Key Derivation

HKDF stands for HMAC-based Key Derivation Function (RFC 5869). It takes a master key and derives unlimited unique child keys from it.

```
Master Key  +  "alice"  +  repo salt  →  HKDF-SHA256  →  Alice's Key
Master Key  +  "bob"    +  repo salt  →  HKDF-SHA256  →  Bob's Key
Master Key  +  "carol"  +  repo salt  →  HKDF-SHA256  →  Carol's Key
```

Each derived key has two critical properties:

**1. It decrypts the same codebase as the master key**
Alice's derived key can decrypt any file that the master key encrypted. They are mathematically compatible, even though they are different keys.

**2. You cannot reverse-engineer the master key from a derived key**
Even if Alice has her derived key and knows the algorithm, she cannot compute the master key or Bob's key. The derivation is one-way.

This means: revoking Carol means invalidating only her derived key — Alice and Bob are completely unaffected.

---

## The Derivation in Code

```go
import "golang.org/x/crypto/hkdf"

// info binds the key to this specific collaborator in this specific repo
info := []byte("sentinel-collab-v1:" + username)

// HKDF: masterKey is the input key material, salt is per-repo random bytes
hkdfReader := hkdf.New(sha256.New, masterKey, salt, info)

// Read 32 bytes = 256-bit derived key
derived := make([]byte, 32)
io.ReadFull(hkdfReader, derived)
```

The `info` parameter is what makes each derived key unique:
- `"sentinel-collab-v1:alice"` → Alice's key
- `"sentinel-collab-v1:bob"` → Bob's key

Same master key, different info → completely different derived keys.

### The Repo Salt

The salt is a 32-byte random value generated once per repo and stored in `.sentinel/keys/repo.salt`. It serves two purposes:

1. **Cross-repo isolation** — if the same master key is accidentally reused in two repos, the different salts mean derived keys from repo A cannot decrypt repo B
2. **Uniqueness** — adds an extra layer of randomness to derivation

---

## The Full Grant Flow

```
sentinel grant alice
        ↓
Load master key from .sentinel/keys/master.key
        ↓
DeriveCollabKey(masterKey, "alice")
        ↓
Save derived key to .sentinel/keys/collaborators/alice.key (0600)
        ↓
Compute key hash (first 8 bytes of SHA-256) → for audit log
        ↓
Add record to .sentinel/collaborators.json:
  {
    "username": "alice",
    "status": "active",
    "granted_at": "2026-03-19T10:00:00Z",
    "granted_by": "thompson",
    "key_hash": "a3f8c2d1"   ← not the key itself, just an audit fingerprint
  }
        ↓
Encode derived key as base64
Output: sentinel:base64encodedkey==:alice
        ↓
You send this string to Alice via Signal or encrypted email
```

---

## The Shareable Key Format

```
sentinel:YWJjZGVmZ2hpams...base64...==:alice
│         │                              │
│         │                              └── username (for audit)
│         └── base64-encoded 32-byte derived key
└── protocol prefix (identifies this as a sentinel key)
```

This format is:
- **Self-describing** — the `sentinel:` prefix prevents confusion with other keys
- **Compact** — base64 is the most efficient text encoding for binary data
- **Attribution-preserving** — username is embedded so the recipient and any auditor knows who the key is for
- **Copy-paste safe** — no special characters that break terminals or messengers

---

## The Collaborator Join Flow

```
# Alice receives: sentinel:YWJjZGVm...==:alice

sentinel collab join --key "sentinel:YWJjZGVm...==:alice"
        ↓
ParseShareableKey() — decode base64, extract username
        ↓
Validate: 32 bytes exactly, correct prefix, non-empty username
        ↓
Write to .sentinel/keys/master.key (Alice's local key file)
        ↓
Verify: LoadAESKey() succeeds
        ↓
✓ Key installed. Alice can now run sentinel pull to decrypt.
```

From Alice's perspective, her derived key IS her master key locally. She doesn't know or need to know whether she has the real master or a derived key — it doesn't matter for daily use.

---

## Revocation — Two Types

### Soft Revoke

```bash
sentinel revoke alice
```

```
Mark alice as "revoked" in .sentinel/collaborators.json
Delete .sentinel/keys/collaborators/alice.key (local copy only)
Done.
```

**What this does:** Removes Alice from Sentinel's registry. Future collaborator grants won't include Alice. Alice no longer shows up in `sentinel whohas`.

**What this does NOT do:** If Alice kept a copy of her key string from the original `sentinel grant` output, she can still decrypt the codebase with it. Soft revoke is administrative, not cryptographic.

Use soft revoke for: collaborators who left amicably, access that expired, reorganisation.

### Hard Revoke (Key Rotation)

```bash
sentinel revoke alice --rotate
```

```
Mark alice as "revoked" in registry
Delete alice's local key file
Generate NEW 256-bit master key (crypto/rand)
Re-derive keys for all ACTIVE collaborators using the new master key
Save new master key to .sentinel/keys/master.key
Output new shareable keys for all active collaborators
```

**What this does:** Alice's derived key from the old master key can no longer decrypt anything — because future commits use the new master key and all files are re-encrypted on next commit. Even if Alice kept her key string, it decrypts nothing new.

**The trade-off:** All active collaborators need to run `sentinel collab join` again with their new keys. This is disruptive but necessary for cryptographic certainty.

Use hard revoke for: terminated employees, suspected key compromise, security incidents.

---

## The Audit Trail

Every grant and revoke is logged in `.sentinel/collaborators.json`:

```json
{
  "collaborators": [
    {
      "username": "alice",
      "status": "active",
      "granted_at": "2026-03-19T10:00:00Z",
      "granted_by": "thompson",
      "key_hash": "a3f8c2d1"
    },
    {
      "username": "carol",
      "status": "revoked",
      "granted_at": "2026-01-15T09:00:00Z",
      "granted_by": "thompson",
      "revoked_at": "2026-03-01T14:00:00Z",
      "key_hash": "9e4b7f12"
    }
  ]
}
```

The `key_hash` field stores the first 8 bytes of the derived key's SHA-256 hash. This is enough to identify a specific key in an audit without storing the key itself. If there's ever a question about which key was used, the hash provides traceability.

---

## sentinel whohas — The Access Audit

```bash
sentinel whohas
```

```
  USERNAME     STATUS       GRANTED AT          KEY HASH
  --------     ------       ----------          --------
  alice        ● active     2026-03-19 10:00    a3f8c2d1
  bob          ● active     2026-03-19 10:05    9e4b7f12
  carol        ✗ revoked    2026-01-15 09:00    7f3a1c84

  Active: 2  |  Total: 3
```

This gives the repo owner a complete picture of who has and had access at any time.

---

## What Collaborators Can and Cannot Do

| Action | Owner | Active Collaborator | Revoked |
|--------|-------|-------------------|---------|
| Decrypt existing files | ✓ | ✓ | ✗ (hard) / ✓ (soft) |
| sentinel commit | ✓ | ✓ | ✗ |
| sentinel grant others | ✓ | ✗ | ✗ |
| sentinel revoke others | ✓ | ✗ | ✗ |
| sentinel whohas | ✓ | ✗ | ✗ |
| sentinel proof | ✓ | ✓ | ✓ |

Derived keys have the same decryption power as the master key — but only the master key holder can manage access.

---

## Security Properties

| Property | Guarantee |
|----------|-----------|
| Collaborator isolation | Alice cannot derive Bob's key |
| Master key secrecy | Alice cannot derive the master key |
| Cross-repo isolation | Repo A keys don't work in Repo B |
| Revocation (soft) | Administrative — immediate in registry |
| Revocation (hard) | Cryptographic — permanent after key rotation |
| Key compromise | Hard revoke + rotation contains the damage |
| Audit trail | Every grant/revoke permanently logged |
