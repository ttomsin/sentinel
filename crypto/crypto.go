package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// ─── Constants ────────────────────────────────────────────────────────────────

const (
	KeyDir      = ".sentinel/keys"
	HashDir     = ".sentinel/hashes"
	AESKeyFile  = ".sentinel/keys/master.key"
	PrivKeyFile = ".sentinel/keys/master.priv"
	PubKeyFile  = ".sentinel/keys/master.pub"
)

// ─── Key Management ──────────────────────────────────────────────────────────

// KeysExist checks if master keys have already been generated
func KeysExist() bool {
	_, err1 := os.Stat(AESKeyFile)
	_, err2 := os.Stat(PrivKeyFile)
	return err1 == nil && err2 == nil
}

// GenerateAESKey creates a cryptographically secure 256-bit (32 byte) AES key
func GenerateAESKey() ([]byte, error) {
	key := make([]byte, 32) // 256 bits
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("failed to generate random key: %w", err)
	}
	return key, nil
}

// GenerateKeyPair creates an Ed25519 private/public key pair
// Ed25519 is modern, fast, and used by SSH and TLS
func GenerateKeyPair() (ed25519.PrivateKey, ed25519.PublicKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate key pair: %w", err)
	}
	return priv, pub, nil
}

// SaveKeys writes all keys to disk with restricted permissions (0600 = owner read/write only)
func SaveKeys(aesKey []byte, privateKey ed25519.PrivateKey, publicKey ed25519.PublicKey) error {
	// AES key — raw bytes as hex
	if err := os.WriteFile(AESKeyFile, []byte(hex.EncodeToString(aesKey)), 0600); err != nil {
		return fmt.Errorf("failed to write AES key: %w", err)
	}

	// Private key — hex encoded
	if err := os.WriteFile(PrivKeyFile, []byte(hex.EncodeToString(privateKey)), 0600); err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}

	// Public key — hex encoded (can be shared)
	if err := os.WriteFile(PubKeyFile, []byte(hex.EncodeToString(publicKey)), 0644); err != nil {
		return fmt.Errorf("failed to write public key: %w", err)
	}

	return nil
}

// SaveAESKey writes a new AES key to disk — used during key rotation
func SaveAESKey(key []byte) error {
	return os.WriteFile(AESKeyFile, []byte(hex.EncodeToString(key)), 0600)
}

// LoadAESKey reads the AES key from disk
func LoadAESKey() ([]byte, error) {
	data, err := os.ReadFile(AESKeyFile)
	if err != nil {
		return nil, fmt.Errorf("could not read AES key: %w", err)
	}

	key, err := hex.DecodeString(string(data))
	if err != nil {
		return nil, fmt.Errorf("invalid AES key format: %w", err)
	}

	if len(key) != 32 {
		return nil, fmt.Errorf("AES key must be 32 bytes, got %d", len(key))
	}

	return key, nil
}

// PublicKeyFingerprint returns a human-readable hex fingerprint of a public key
func PublicKeyFingerprint(pub ed25519.PublicKey) (string, error) {
	hash := sha256.Sum256(pub)
	fingerprint := hex.EncodeToString(hash[:])

	// Format like SSH: ab:cd:ef:12:...
	formatted := ""
	for i := 0; i < len(fingerprint); i += 2 {
		if i > 0 {
			formatted += ":"
		}
		formatted += fingerprint[i : i+2]
	}
	return "SHA256:" + formatted[:47] + "...", nil
}

// ─── Hashing ─────────────────────────────────────────────────────────────────

// FileHash holds a file path and its SHA-256 hash
type FileHash struct {
	Path      string    `json:"path"`
	Hash      string    `json:"hash"`
	Size      int64     `json:"size"`
	Timestamp time.Time `json:"timestamp"`
}

// HashRecord is saved to .sentinel/hashes/
type HashRecord struct {
	Version   string     `json:"sentinel_version"`
	Timestamp time.Time  `json:"timestamp"`
	Files     []FileHash `json:"files"`
	RootHash  string     `json:"root_hash"` // Hash of all hashes combined
}

// HashFiles computes SHA-256 for each file and returns a record
func HashFiles(paths []string) ([]FileHash, error) {
	var hashes []FileHash

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			// File might be deleted — skip it
			continue
		}

		hash := sha256.Sum256(data)

		info, _ := os.Stat(path)
		size := int64(0)
		if info != nil {
			size = info.Size()
		}

		hashes = append(hashes, FileHash{
			Path:      path,
			Hash:      hex.EncodeToString(hash[:]),
			Size:      size,
			Timestamp: time.Now().UTC(),
		})
	}

	return hashes, nil
}

// SaveHashes writes a JSON hash record to .sentinel/hashes/ and returns (filename, rootHash, error)
func SaveHashes(hashes []FileHash, ts time.Time) (string, string, error) {
	// Compute root hash — hash of all individual hashes combined
	combined := ""
	for _, h := range hashes {
		combined += h.Hash
	}
	rootHash := sha256.Sum256([]byte(combined))
	rootHashHex := hex.EncodeToString(rootHash[:])

	record := HashRecord{
		Version:   "1.0",
		Timestamp: ts,
		Files:     hashes,
		RootHash:  rootHashHex,
	}

	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return "", "", err
	}

	filename := filepath.Join(HashDir, fmt.Sprintf("%d.json", ts.Unix()))
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return "", "", err
	}

	return filename, rootHashHex, nil
}

// ─── Encryption ──────────────────────────────────────────────────────────────

// EncryptFiles encrypts each file in place using AES-256-GCM
// AES-GCM provides both encryption AND authentication (tamper detection)
func EncryptFiles(paths []string, key []byte) error {
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %w", err)
	}

	for _, path := range paths {
		if err := encryptFile(path, gcm); err != nil {
			return fmt.Errorf("failed to encrypt %s: %w", path, err)
		}
	}

	return nil
}

// encryptFile encrypts a single file in place
func encryptFile(path string, gcm cipher.AEAD) error {
	plaintext, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// Generate a random nonce for each file
	// CRITICAL: Never reuse a nonce with the same key
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt: ciphertext = nonce + GCM(plaintext)
	// We prepend the nonce so we can recover it during decryption
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	// Overwrite the file with encrypted content
	return os.WriteFile(path, ciphertext, 0644)
}

// DecryptFiles decrypts each file in place using AES-256-GCM
func DecryptFiles(paths []string, key []byte) error {
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %w", err)
	}

	for _, path := range paths {
		if err := decryptFile(path, gcm); err != nil {
			return fmt.Errorf("failed to decrypt %s: %w", path, err)
		}
	}

	return nil
}

// decryptFile decrypts a single file in place
func decryptFile(path string, gcm cipher.AEAD) error {
	ciphertext, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return fmt.Errorf("file too short to be encrypted: %s", path)
	}

	// Split nonce and actual ciphertext
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Decrypt and verify authenticity
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return fmt.Errorf("decryption failed (wrong key or tampered file): %w", err)
	}

	return os.WriteFile(path, plaintext, 0644)
}
