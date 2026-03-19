// Package collab handles collaborator access control for Sentinel.
//
// How it works:
//   - The repo owner holds a master AES-256 key
//   - For each collaborator, a unique derived key is generated using HKDF
//   - Derived keys decrypt the same codebase as the master key
//   - Revoking a collaborator = soft revoke (registry) + optional hard revoke (key rotation)
//   - Key sharing happens out-of-band (owner exports base64 key, sends via Signal/email)
//
// Cryptography used:
//   - HKDF (HMAC-based Key Derivation Function) — RFC 5869
//   - SHA-256 as the HKDF hash function
//   - Each derived key is tied to: master key + collaborator username + repo salt
package collab

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/hkdf"
)

const (
	CollabDir    = ".sentinel/keys/collaborators"
	RegistryFile = ".sentinel/collaborators.json"
	SaltFile     = ".sentinel/keys/repo.salt"
)

// CollabRecord stores metadata about a collaborator's access
type CollabRecord struct {
	Username  string     `json:"username"`
	KeyFile   string     `json:"key_file"`
	GrantedAt time.Time  `json:"granted_at"`
	GrantedBy string     `json:"granted_by"`
	Status    string     `json:"status"` // "active", "revoked"
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	KeyHash   string     `json:"key_hash"` // SHA-256 of the derived key (for audit)
}

// Registry holds all collaborator records for this repo
type Registry struct {
	Version       string         `json:"sentinel_version"`
	RepoID        string         `json:"repo_id"` // unique per repo
	Collaborators []CollabRecord `json:"collaborators"`
}

// ─── Key Derivation ───────────────────────────────────────────────────────────

// DeriveCollabKey derives a unique AES-256 key for a collaborator using HKDF.
//
// HKDF takes:
//   - masterKey: the repo owner's AES-256 key (the "input key material")
//   - salt: a random per-repo salt (prevents cross-repo key reuse)
//   - info: collaborator username (makes each derived key unique per person)
//
// The same inputs always produce the same output — deterministic but secure.
// A collaborator cannot reverse-engineer the master key from their derived key.
func DeriveCollabKey(masterKey []byte, username string) ([]byte, error) {
	salt, err := loadOrCreateSalt()
	if err != nil {
		return nil, fmt.Errorf("failed to load repo salt: %w", err)
	}

	// HKDF-SHA256: master key → derived key
	// info binds the key to this specific collaborator in this specific repo
	info := []byte("sentinel-collab-v1:" + username)

	hkdfReader := hkdf.New(sha256.New, masterKey, salt, info)

	derived := make([]byte, 32) // 256-bit derived key
	if _, err := io.ReadFull(hkdfReader, derived); err != nil {
		return nil, fmt.Errorf("HKDF derivation failed: %w", err)
	}

	return derived, nil
}

// ─── Grant Access ─────────────────────────────────────────────────────────────

// GrantAccess derives a key for a collaborator, saves it locally, and returns
// the base64-encoded key string to share with the collaborator out-of-band.
func GrantAccess(masterKey []byte, username string, grantedBy string) (*CollabRecord, string, error) {
	// Create collaborator key directory
	if err := os.MkdirAll(CollabDir, 0700); err != nil {
		return nil, "", fmt.Errorf("failed to create collab key dir: %w", err)
	}

	// Check if collaborator already has access
	registry, _ := LoadRegistry()
	for _, r := range registry.Collaborators {
		if r.Username == username && r.Status == "active" {
			return nil, "", fmt.Errorf("collaborator '%s' already has active access\nUse 'sentinel revoke %s' first to regenerate", username, username)
		}
	}

	// Derive the collaborator's key
	derivedKey, err := DeriveCollabKey(masterKey, username)
	if err != nil {
		return nil, "", fmt.Errorf("key derivation failed: %w", err)
	}

	// Save derived key to .sentinel/keys/collaborators/<username>.key
	keyFile := filepath.Join(CollabDir, username+".key")
	keyHex := fmt.Sprintf("%x", derivedKey)
	if err := os.WriteFile(keyFile, []byte(keyHex), 0600); err != nil {
		return nil, "", fmt.Errorf("failed to save derived key: %w", err)
	}

	// Compute key hash for audit log (we store the hash, not the key, in the registry)
	hash := sha256.Sum256(derivedKey)
	keyHash := fmt.Sprintf("%x", hash[:8]) // first 8 bytes = 16 hex chars, enough for audit

	// Create registry record
	record := CollabRecord{
		Username:  username,
		KeyFile:   keyFile,
		GrantedAt: time.Now().UTC(),
		GrantedBy: grantedBy,
		Status:    "active",
		KeyHash:   keyHash,
	}

	// Add to registry
	if err := addToRegistry(record); err != nil {
		return nil, "", fmt.Errorf("failed to update registry: %w", err)
	}

	// Encode key as base64 for sharing — compact and copy-pasteable
	// Format: sentinel:<base64key>:<username>
	// The username is included so the recipient knows who it's for
	keyBase64 := base64.StdEncoding.EncodeToString(derivedKey)
	shareableKey := fmt.Sprintf("sentinel:%s:%s", keyBase64, username)

	return &record, shareableKey, nil
}

// ─── Revoke Access ────────────────────────────────────────────────────────────

// RevokeAccess soft-revokes a collaborator's access.
// Soft revoke: marks as revoked in registry, deletes their local key file.
// The collaborator's copy of the key still works until a hard revoke (key rotation).
func RevokeAccess(username string) error {
	registry, err := LoadRegistry()
	if err != nil {
		return fmt.Errorf("no registry found")
	}

	found := false
	now := time.Now().UTC()

	for i, r := range registry.Collaborators {
		if r.Username == username && r.Status == "active" {
			registry.Collaborators[i].Status = "revoked"
			registry.Collaborators[i].RevokedAt = &now
			found = true

			// Delete the local key file
			_ = os.Remove(r.KeyFile)
		}
	}

	if !found {
		return fmt.Errorf("no active access found for '%s'", username)
	}

	return saveRegistry(registry)
}

// RotateKeys performs a hard revoke by generating a new master key and
// re-deriving keys only for active (non-revoked) collaborators.
// This makes revoked keys permanently invalid — even if the collaborator
// kept a copy of their old derived key.
//
// This is called after RevokeAccess when the owner wants a hard revoke.
func RotateKeys(oldMasterKey []byte) ([]byte, error) {
	// Generate a completely new master key
	newMasterKey := make([]byte, 32)
	if _, err := rand.Read(newMasterKey); err != nil {
		return nil, fmt.Errorf("failed to generate new master key: %w", err)
	}

	// Get list of currently active collaborators
	registry, err := LoadRegistry()
	if err != nil {
		// No registry — just return the new key
		return newMasterKey, nil
	}

	// Re-derive keys for all active collaborators using the new master key
	for _, r := range registry.Collaborators {
		if r.Status != "active" {
			continue
		}

		newDerived, err := DeriveCollabKey(newMasterKey, r.Username)
		if err != nil {
			continue
		}

		// Overwrite their key file with new derived key
		keyHex := fmt.Sprintf("%x", newDerived)
		_ = os.WriteFile(r.KeyFile, []byte(keyHex), 0600)
	}

	return newMasterKey, nil
}

// ─── Collaborator Side — Joining a Repo ──────────────────────────────────────

// ParseShareableKey parses a shareable key string and returns the raw key bytes
// and the username it was issued to.
// Format: sentinel:<base64key>:<username>
func ParseShareableKey(shareableKey string) ([]byte, string, error) {
	const prefix = "sentinel:"

	if len(shareableKey) < len(prefix) || shareableKey[:len(prefix)] != prefix {
		return nil, "", fmt.Errorf("invalid key format — should start with 'sentinel:'")
	}

	rest := shareableKey[len(prefix):]

	// Find the last colon to split base64 and username
	lastColon := -1
	for i := len(rest) - 1; i >= 0; i-- {
		if rest[i] == ':' {
			lastColon = i
			break
		}
	}

	if lastColon == -1 {
		return nil, "", fmt.Errorf("invalid key format — missing username component")
	}

	keyBase64 := rest[:lastColon]
	username := rest[lastColon+1:]

	if username == "" {
		return nil, "", fmt.Errorf("invalid key format — empty username")
	}

	keyBytes, err := base64.StdEncoding.DecodeString(keyBase64)
	if err != nil {
		return nil, "", fmt.Errorf("invalid key encoding: %w", err)
	}

	if len(keyBytes) != 32 {
		return nil, "", fmt.Errorf("invalid key length: expected 32 bytes, got %d", len(keyBytes))
	}

	return keyBytes, username, nil
}

// InstallCollabKey installs a received collaborator key on the local machine.
// Called by the collaborator after receiving the key from the repo owner.
func InstallCollabKey(shareableKey string) (string, error) {
	keyBytes, username, err := ParseShareableKey(shareableKey)
	if err != nil {
		return "", err
	}

	// Save as the local AES key (overwrites or creates)
	// The collaborator uses this key for all decryption
	collabKeyFile := ".sentinel/keys/master.key"

	if err := os.MkdirAll(".sentinel/keys", 0700); err != nil {
		return "", fmt.Errorf("failed to create keys directory: %w", err)
	}

	keyHex := fmt.Sprintf("%x", keyBytes)
	if err := os.WriteFile(collabKeyFile, []byte(keyHex), 0600); err != nil {
		return "", fmt.Errorf("failed to install key: %w", err)
	}

	return username, nil
}

// ─── Registry Management ─────────────────────────────────────────────────────

// LoadRegistry loads the collaborator registry from disk
func LoadRegistry() (*Registry, error) {
	data, err := os.ReadFile(RegistryFile)
	if err != nil {
		return &Registry{Version: "1.0"}, nil
	}

	var registry Registry
	if err := json.Unmarshal(data, &registry); err != nil {
		return nil, fmt.Errorf("corrupted registry: %w", err)
	}

	return &registry, nil
}

// ListActive returns all currently active collaborators
func ListActive() ([]CollabRecord, error) {
	registry, err := LoadRegistry()
	if err != nil {
		return nil, err
	}

	var active []CollabRecord
	for _, r := range registry.Collaborators {
		if r.Status == "active" {
			active = append(active, r)
		}
	}
	return active, nil
}

func saveRegistry(registry *Registry) error {
	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(RegistryFile, data, 0644)
}

func addToRegistry(record CollabRecord) error {
	registry, _ := LoadRegistry()
	if registry == nil {
		registry = &Registry{Version: "1.0"}
	}
	registry.Collaborators = append(registry.Collaborators, record)
	return saveRegistry(registry)
}

// ─── Salt Management ─────────────────────────────────────────────────────────

// loadOrCreateSalt loads the per-repo salt, creating it if it doesn't exist.
// The salt ensures derived keys from this repo can't be used on other repos
// even if the master key is reused.
func loadOrCreateSalt() ([]byte, error) {
	// Try to load existing salt
	data, err := os.ReadFile(SaltFile)
	if err == nil && len(data) == 32 {
		return data, nil
	}

	// Generate a new random salt
	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}

	// Save it
	if err := os.MkdirAll(filepath.Dir(SaltFile), 0700); err != nil {
		return nil, err
	}

	if err := os.WriteFile(SaltFile, salt, 0600); err != nil {
		return nil, fmt.Errorf("failed to save salt: %w", err)
	}

	return salt, nil
}
