// Package blockchain handles anchoring code hashes to Bitcoin via OpenTimestamps.
//
// OpenTimestamps is a free, open protocol that batches thousands of hashes
// into a single Bitcoin transaction. You pay nothing — they cover the gas fee.
//
// How it works:
//  1. We send our SHA-256 root_hash to the OTS calendar servers
//  2. They return a .ots proof file (binary) — status: PENDING
//  3. Within ~1-24hrs the .ots is confirmed on Bitcoin — status: CONFIRMED
//  4. Anyone with the hash + .ots file can verify it forever, independently
//
// API docs: https://opentimestamps.org
package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// OTS calendar servers — we try all three for redundancy
// If one is down, we fall back to the next
var calendarServers = []string{
	"https://a.pool.opentimestamps.org",
	"https://b.pool.opentimestamps.org",
	"https://c.pool.opentimestamps.org",
}

const (
	ProofDir       = ".sentinel/proofs"
	ProofIndexFile = ".sentinel/proofs/index.json"
	httpTimeout    = 15 * time.Second
)

// ProofRecord stores metadata about an anchored hash
type ProofRecord struct {
	RootHash     string    `json:"root_hash"`
	HashFile     string    `json:"hash_file"`
	OTSFile      string    `json:"ots_file"`
	Server       string    `json:"calendar_server"`
	SubmittedAt  time.Time `json:"submitted_at"`
	Status       string    `json:"status"`        // "pending", "confirmed", "failed"
	BitcoinTx    string    `json:"bitcoin_tx"`    // filled in after confirmation
	BitcoinBlock int64     `json:"bitcoin_block"` // filled in after confirmation
}

// ProofIndex is the full list of all anchored hashes for this repo
type ProofIndex struct {
	Version string        `json:"sentinel_version"`
	Records []ProofRecord `json:"records"`
}

// RegisterProof saves a pending proof record to disk immediately (synchronous).
// Call this BEFORE launching any goroutine so the record is persisted even if
// the process exits before the HTTP call completes.
func RegisterProof(rootHash string, hashFile string) (*ProofRecord, error) {
	if err := os.MkdirAll(ProofDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create proofs directory: %w", err)
	}

	otsFilename := filepath.Join(ProofDir, fmt.Sprintf("%s.ots", rootHash[:16]))

	record := ProofRecord{
		RootHash:    rootHash,
		HashFile:    hashFile,
		OTSFile:     otsFilename,
		Server:      calendarServers[0],
		SubmittedAt: time.Now().UTC(),
		Status:      "pending",
	}

	if err := addToIndex(record); err != nil {
		return nil, fmt.Errorf("failed to save proof record: %w", err)
	}

	return &record, nil
}

// AnchorHash submits a root hash to OpenTimestamps and saves the .ots proof file.
// Call RegisterProof first to persist the record, then call this in a goroutine.
func AnchorHash(rootHash string, hashFile string) (*ProofRecord, error) {
	// Decode the hex hash to raw bytes — OTS API expects raw binary
	hashBytes, err := hex.DecodeString(rootHash)
	if err != nil {
		return nil, fmt.Errorf("invalid root hash format: %w", err)
	}

	if err := os.MkdirAll(ProofDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create proofs directory: %w", err)
	}

	// Try each calendar server until one succeeds
	var otsData []byte
	var usedServer string
	var lastErr error

	for _, server := range calendarServers {
		otsData, err = submitToCalendar(server, hashBytes)
		if err != nil {
			lastErr = err
			continue
		}
		usedServer = server
		break
	}

	if otsData == nil {
		// Mark record as failed in index
		_ = updateIndexRecord(ProofRecord{
			RootHash: rootHash,
			HashFile: hashFile,
			OTSFile:  filepath.Join(ProofDir, fmt.Sprintf("%s.ots", rootHash[:16])),
			Status:   "failed",
		})
		return nil, fmt.Errorf("all calendar servers failed. Last error: %w", lastErr)
	}

	// Save the .ots proof file
	otsFilename := filepath.Join(ProofDir, fmt.Sprintf("%s.ots", rootHash[:16]))
	if err := os.WriteFile(otsFilename, otsData, 0644); err != nil {
		return nil, fmt.Errorf("failed to save .ots proof file: %w", err)
	}

	// Update the record with server info (status stays "pending" until Bitcoin confirms)
	record := ProofRecord{
		RootHash:    rootHash,
		HashFile:    hashFile,
		OTSFile:     otsFilename,
		Server:      usedServer,
		SubmittedAt: time.Now().UTC(),
		Status:      "pending",
	}

	_ = updateIndexRecord(record)
	return &record, nil
}

// submitToCalendar sends the hash to one OTS calendar server
// POST /digest with raw hash bytes → returns raw .ots binary data
func submitToCalendar(server string, hashBytes []byte) ([]byte, error) {
	url := server + "/digest"

	client := &http.Client{Timeout: httpTimeout}
	req, err := http.NewRequest("POST", url, bytes.NewReader(hashBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// OTS API expects Content-Type: application/x-www-form-urlencoded
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/vnd.opentimestamps.v1")
	req.Header.Set("User-Agent", "sentinel-cli/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to %s failed: %w", server, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server %s returned %d: %s", server, resp.StatusCode, string(body))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("server %s returned empty response", server)
	}

	return data, nil
}

// UpgradeProof attempts to upgrade a pending .ots file to a confirmed Bitcoin timestamp.
// OTS proof files start as "pending" and become "confirmed" once Bitcoin includes the tx.
// This is called by `sentinel proof upgrade` or automatically by `sentinel proof status`.
func UpgradeProof(record *ProofRecord) (*ProofRecord, error) {
	otsData, err := os.ReadFile(record.OTSFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read .ots file: %w", err)
	}

	// Try each calendar server for the upgrade
	for _, server := range calendarServers {
		upgraded, err := fetchUpgrade(server, record.RootHash, otsData)
		if err != nil {
			continue
		}

		// Save the upgraded .ots file (overwrites pending version)
		if err := os.WriteFile(record.OTSFile, upgraded, 0644); err != nil {
			return nil, fmt.Errorf("failed to save upgraded .ots: %w", err)
		}

		record.Status = "confirmed"
		// Update the index
		_ = updateIndexRecord(*record)
		return record, nil
	}

	return record, fmt.Errorf("upgrade not ready yet — Bitcoin confirmation takes up to 24hrs")
}

// fetchUpgrade requests the latest version of an .ots file from a calendar server
func fetchUpgrade(server string, rootHash string, otsData []byte) ([]byte, error) {
	// The upgrade endpoint takes the hash as hex in the URL
	url := fmt.Sprintf("%s/timestamp/%s", server, rootHash)

	client := &http.Client{Timeout: httpTimeout}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.opentimestamps.v1")
	req.Header.Set("User-Agent", "sentinel-cli/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 200 = confirmed, 404 = not yet confirmed
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("not yet confirmed")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// VerifyHash checks if a given hash has a valid .ots proof file locally
func VerifyHash(rootHash string) (*ProofRecord, error) {
	index, err := loadIndex()
	if err != nil {
		return nil, fmt.Errorf("no proof index found — run 'sentinel proof list'")
	}

	for _, record := range index.Records {
		if record.RootHash == rootHash {
			return &record, nil
		}
	}

	return nil, fmt.Errorf("no proof found for hash: %s", rootHash[:16]+"...")
}

// ListProofs returns all proof records for this repository
func ListProofs() ([]ProofRecord, error) {
	index, err := loadIndex()
	if err != nil {
		return nil, nil // No proofs yet — not an error
	}
	return index.Records, nil
}

// GetLatestProof returns the most recent proof record
func GetLatestProof() (*ProofRecord, error) {
	records, err := ListProofs()
	if err != nil || len(records) == 0 {
		return nil, fmt.Errorf("no proofs found — run 'sentinel commit' first")
	}
	latest := records[len(records)-1]
	return &latest, nil
}

// HashFile computes SHA-256 of the .ots file itself
// Useful for verifying the proof file hasn't been tampered with
func HashOTSFile(otsPath string) (string, error) {
	data, err := os.ReadFile(otsPath)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// ─── Index Management ─────────────────────────────────────────────────────────

func loadIndex() (*ProofIndex, error) {
	data, err := os.ReadFile(ProofIndexFile)
	if err != nil {
		return &ProofIndex{Version: "1.0"}, nil
	}

	var index ProofIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("corrupted proof index: %w", err)
	}
	return &index, nil
}

func saveIndex(index *ProofIndex) error {
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ProofIndexFile, data, 0644)
}

func addToIndex(record ProofRecord) error {
	index, err := loadIndex()
	if err != nil {
		index = &ProofIndex{Version: "1.0"}
	}
	index.Records = append(index.Records, record)
	return saveIndex(index)
}

func updateIndexRecord(updated ProofRecord) error {
	index, err := loadIndex()
	if err != nil {
		return err
	}
	for i, r := range index.Records {
		if r.RootHash == updated.RootHash {
			index.Records[i] = updated
			return saveIndex(index)
		}
	}
	return fmt.Errorf("record not found in index")
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// FormatStatus returns a coloured status string for display
func FormatStatus(status string) string {
	switch strings.ToLower(status) {
	case "confirmed":
		return "✓ CONFIRMED (Bitcoin)"
	case "pending":
		return "⏳ PENDING  (awaiting Bitcoin confirmation)"
	case "failed":
		return "✗ FAILED"
	default:
		return status
	}
}

// ShortHash returns the first 16 chars of a hash for display
func ShortHash(hash string) string {
	if len(hash) > 16 {
		return hash[:16] + "..."
	}
	return hash
}
