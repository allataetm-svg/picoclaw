package pairing

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type PairingManager struct {
	mu          sync.RWMutex
	pending     map[string]*PendingPairing
	approved    map[string]*ApprovedPairing
	storagePath string
	codeLength  int
	codeExpiry  time.Duration
}

type PendingPairing struct {
	Channel    string
	SenderID   string
	Code       string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	SenderName string
}

type ApprovedPairing struct {
	Channel    string
	SenderID   string
	ApprovedAt time.Time
	SenderName string
}

func NewPairingManager(storagePath string) *PairingManager {
	pm := &PairingManager{
		pending:     make(map[string]*PendingPairing),
		approved:    make(map[string]*ApprovedPairing),
		storagePath: storagePath,
		codeLength:  6,
		codeExpiry:  5 * time.Minute,
	}

	if storagePath != "" {
		os.MkdirAll(storagePath, 0o755)
		pm.load()
	}

	return pm
}

func (pm *PairingManager) load() {
	path := filepath.Join(pm.storagePath, "approved.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	var approvals map[string]*ApprovedPairing
	if err := json.Unmarshal(data, &approvals); err != nil {
		return
	}

	pm.approved = approvals
}

func (pm *PairingManager) save() {
	if pm.storagePath == "" {
		return
	}

	path := filepath.Join(pm.storagePath, "approved.json")
	data, err := json.MarshalIndent(pm.approved, "", "  ")
	if err != nil {
		return
	}

	os.WriteFile(path, data, 0o644)
}

func (pm *PairingManager) GenerateCode(channel, senderID, senderName string) (string, error) {
	code, err := pm.generateSecureCode()
	if err != nil {
		return "", err
	}

	key := pm.makeKey(channel, senderID)

	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.pending[key] = &PendingPairing{
		Channel:    channel,
		SenderID:   senderID,
		Code:       code,
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(pm.codeExpiry),
		SenderName: senderName,
	}

	return code, nil
}

func (pm *PairingManager) GenerateCodeForApproval(channel, senderID, senderName string) string {
	code, err := pm.GenerateCode(channel, senderID, senderName)
	if err != nil {
		return ""
	}
	return code
}

func (pm *PairingManager) generateSecureCode() (string, error) {
	const digits = "0123456789"
	code := make([]byte, pm.codeLength)

	for i := range code {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			return "", err
		}
		code[i] = digits[n.Int64()]
	}

	return string(code), nil
}

func (pm *PairingManager) Approve(channel, senderID, code string) (bool, error) {
	key := pm.makeKey(channel, senderID)

	pm.mu.Lock()
	defer pm.mu.Unlock()

	pending, ok := pm.pending[key]
	if !ok {
		return false, fmt.Errorf("no pending pairing request")
	}

	if time.Now().After(pending.ExpiresAt) {
		delete(pm.pending, key)
		return false, fmt.Errorf("pairing code expired")
	}

	if pending.Code != code {
		return false, fmt.Errorf("invalid pairing code")
	}

	pm.approved[key] = &ApprovedPairing{
		Channel:    channel,
		SenderID:   senderID,
		ApprovedAt: time.Now(),
		SenderName: pending.SenderName,
	}

	delete(pm.pending, key)
	pm.save()

	return true, nil
}

func (pm *PairingManager) IsApproved(channel, senderID string) bool {
	key := pm.makeKey(channel, senderID)

	pm.mu.RLock()
	defer pm.mu.RUnlock()

	_, ok := pm.approved[key]
	return ok
}

func (pm *PairingManager) IsPending(channel, senderID string) bool {
	key := pm.makeKey(channel, senderID)

	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if pending, ok := pm.pending[key]; ok {
		if time.Now().Before(pending.ExpiresAt) {
			return true
		}
	}

	return false
}

func (pm *PairingManager) GetPendingPairing(channel, senderID string) *PendingPairing {
	key := pm.makeKey(channel, senderID)

	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if pending, ok := pm.pending[key]; ok {
		if time.Now().Before(pending.ExpiresAt) {
			return pending
		}
	}

	return nil
}

func (pm *PairingManager) Revoke(channel, senderID string) bool {
	key := pm.makeKey(channel, senderID)

	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, ok := pm.approved[key]; ok {
		delete(pm.approved, key)
		pm.save()
		return true
	}

	return false
}

func (pm *PairingManager) ListApproved() []*ApprovedPairing {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make([]*ApprovedPairing, 0, len(pm.approved))
	for _, p := range pm.approved {
		result = append(result, p)
	}

	return result
}

func (pm *PairingManager) ListPending() []*PendingPairing {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make([]*PendingPairing, 0)
	now := time.Now()

	for _, p := range pm.pending {
		if now.Before(p.ExpiresAt) {
			result = append(result, p)
		}
	}

	return result
}

func (pm *PairingManager) Cleanup() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	now := time.Now()
	for key, p := range pm.pending {
		if now.After(p.ExpiresAt) {
			delete(pm.pending, key)
		}
	}
}

func (pm *PairingManager) makeKey(channel, senderID string) string {
	return strings.ToLower(channel + ":" + senderID)
}
