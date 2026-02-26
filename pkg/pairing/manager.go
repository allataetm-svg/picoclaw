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
	usedCodes   map[string]bool
	storagePath string
	codeLength  int
	codeExpiry  time.Duration
	rateLimits  map[string]rateLimitEntry
}

type rateLimitEntry struct {
	count     int
	resetTime time.Time
}

const (
	rateLimitMax    = 3
	rateLimitWindow = 60 * time.Second
)

type PendingPairing struct {
	Channel    string    `json:"channel"`
	SenderID   string    `json:"sender_id"`
	Code       string    `json:"code"`
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	SenderName string    `json:"sender_name"`
}

type ApprovedPairing struct {
	Channel    string    `json:"channel"`
	SenderID   string    `json:"sender_id"`
	ApprovedAt time.Time `json:"approved_at"`
	SenderName string    `json:"sender_name"`
}

func NewPairingManager(storagePath string) *PairingManager {
	pm := &PairingManager{
		pending:     make(map[string]*PendingPairing),
		approved:    make(map[string]*ApprovedPairing),
		usedCodes:   make(map[string]bool),
		storagePath: storagePath,
		codeLength:  6,
		codeExpiry:  5 * time.Minute,
		rateLimits:  make(map[string]rateLimitEntry),
	}

	if storagePath != "" {
		os.MkdirAll(storagePath, 0o755)
		pm.load()
	}

	return pm
}

func (pm *PairingManager) load() {
	pm.loadApproved()
	pm.loadPending()
}

func (pm *PairingManager) loadApproved() {
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

func (pm *PairingManager) loadPending() {
	path := filepath.Join(pm.storagePath, "pending.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	var pending map[string]*PendingPairing
	if err := json.Unmarshal(data, &pending); err != nil {
		return
	}

	now := time.Now()
	for key, p := range pending {
		if now.After(p.ExpiresAt) {
			delete(pending, key)
		}
	}

	pm.pending = pending
}

func (pm *PairingManager) save() {
	if pm.storagePath == "" {
		return
	}

	pm.saveApproved()
	pm.savePending()
}

func (pm *PairingManager) saveApproved() {
	path := filepath.Join(pm.storagePath, "approved.json")
	data, err := json.MarshalIndent(pm.approved, "", "  ")
	if err != nil {
		return
	}

	os.WriteFile(path, data, 0o644)
}

func (pm *PairingManager) savePending() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	now := time.Now()
	for key, p := range pm.pending {
		if now.After(p.ExpiresAt) {
			delete(pm.pending, key)
		}
	}

	path := filepath.Join(pm.storagePath, "pending.json")
	data, err := json.MarshalIndent(pm.pending, "", "  ")
	if err != nil {
		return
	}

	os.WriteFile(path, data, 0o644)
}

func (pm *PairingManager) GenerateCode(channel, senderID, senderName string) (string, error) {
	rateKey := pm.makeKey(channel, senderID)
	if !pm.checkRateLimit(rateKey) {
		return "", fmt.Errorf("too many requests, please wait before requesting a new code")
	}

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

	pm.savePending()

	return code, nil
}

func (pm *PairingManager) checkRateLimit(key string) bool {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	now := time.Now()
	entry, exists := pm.rateLimits[key]

	if !exists || now.After(entry.resetTime) {
		pm.rateLimits[key] = rateLimitEntry{
			count:     1,
			resetTime: now.Add(rateLimitWindow),
		}
		return true
	}

	if entry.count >= rateLimitMax {
		return false
	}

	entry.count++
	pm.rateLimits[key] = entry
	return true
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

	if pm.usedCodes[code] {
		return false, fmt.Errorf("pairing code already used")
	}

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

	pm.usedCodes[code] = true
	delete(pm.pending, key)
	pm.save()

	return true, nil
}

func (pm *PairingManager) ApproveByCode(code string) (bool, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.usedCodes[code] {
		return false, fmt.Errorf("pairing code already used")
	}

	now := time.Now()
	var foundKey string
	var pending *PendingPairing

	for key, p := range pm.pending {
		if p.Code == code && now.Before(p.ExpiresAt) {
			foundKey = key
			pending = p
			break
		}
	}

	if pending == nil {
		return false, fmt.Errorf("invalid or expired pairing code")
	}

	pm.approved[foundKey] = &ApprovedPairing{
		Channel:    pending.Channel,
		SenderID:   pending.SenderID,
		ApprovedAt: time.Now(),
		SenderName: pending.SenderName,
	}

	pm.usedCodes[code] = true
	delete(pm.pending, foundKey)
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

func (pm *PairingManager) GetPendingByCode(code string) *PendingPairing {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	now := time.Now()
	for _, pending := range pm.pending {
		if pending.Code == code && now.Before(pending.ExpiresAt) {
			return pending
		}
	}

	return nil
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
