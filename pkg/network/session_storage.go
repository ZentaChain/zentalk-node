package network

import (
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zentalk/protocol/pkg/protocol"
)

// SessionStorage handles persistence of X3DH and ratchet session state
type SessionStorage struct {
	storageDir string
}

// NewSessionStorage creates a new session storage
func NewSessionStorage(storageDir string) (*SessionStorage, error) {
	// Create storage directory if it doesn't exist
	if err := os.MkdirAll(storageDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	return &SessionStorage{
		storageDir: storageDir,
	}, nil
}

// X3DHState represents the serializable X3DH state
type X3DHState struct {
	IdentityKeyPair *protocol.IdentityKeyPair          `json:"identity"`
	SignedPreKey    *protocol.SignedPreKeyPrivate      `json:"signed_prekey"`
	OneTimePreKeys  map[string]*protocol.OneTimePreKeyPrivate `json:"one_time_prekeys"` // key is string(uint32)
	RegistrationID  uint32                              `json:"registration_id"`
}

// RatchetSessionData represents a serializable ratchet session
type RatchetSessionData struct {
	Address string                   `json:"address"` // hex-encoded
	State   *protocol.RatchetState   `json:"state"`
}

// SaveX3DHState saves the X3DH state to disk
func (s *SessionStorage) SaveX3DHState(state *X3DHState) error {
	filePath := filepath.Join(s.storageDir, "x3dh_state.json")

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal X3DH state: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write X3DH state: %w", err)
	}

	return nil
}

// LoadX3DHState loads the X3DH state from disk
func (s *SessionStorage) LoadX3DHState() (*X3DHState, error) {
	filePath := filepath.Join(s.storageDir, "x3dh_state.json")

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No state file exists yet
		}
		return nil, fmt.Errorf("failed to read X3DH state: %w", err)
	}

	var state X3DHState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal X3DH state: %w", err)
	}

	return &state, nil
}

// SaveRatchetSession saves a single ratchet session
func (s *SessionStorage) SaveRatchetSession(addr protocol.Address, session *protocol.RatchetState) error {
	// Load all sessions
	sessions, err := s.LoadAllRatchetSessions()
	if err != nil {
		return err
	}

	// Update the specific session
	addrHex := hex.EncodeToString(addr[:])
	sessions[addrHex] = session

	// Save all sessions
	return s.saveAllRatchetSessions(sessions)
}

// LoadRatchetSession loads a single ratchet session
func (s *SessionStorage) LoadRatchetSession(addr protocol.Address) (*protocol.RatchetState, error) {
	sessions, err := s.LoadAllRatchetSessions()
	if err != nil {
		return nil, err
	}

	addrHex := hex.EncodeToString(addr[:])
	session, ok := sessions[addrHex]
	if !ok {
		return nil, nil // Session doesn't exist
	}

	return session, nil
}

// LoadAllRatchetSessions loads all ratchet sessions
func (s *SessionStorage) LoadAllRatchetSessions() (map[string]*protocol.RatchetState, error) {
	filePath := filepath.Join(s.storageDir, "ratchet_sessions.gob")

	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]*protocol.RatchetState), nil
		}
		return nil, fmt.Errorf("failed to open ratchet sessions: %w", err)
	}
	defer file.Close()

	var sessions map[string]*protocol.RatchetState
	decoder := gob.NewDecoder(file)
	if err := decoder.Decode(&sessions); err != nil {
		return nil, fmt.Errorf("failed to decode ratchet sessions: %w", err)
	}

	return sessions, nil
}

// saveAllRatchetSessions saves all ratchet sessions
func (s *SessionStorage) saveAllRatchetSessions(sessions map[string]*protocol.RatchetState) error {
	filePath := filepath.Join(s.storageDir, "ratchet_sessions.gob")

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create ratchet sessions file: %w", err)
	}
	defer file.Close()

	encoder := gob.NewEncoder(file)
	if err := encoder.Encode(sessions); err != nil {
		return fmt.Errorf("failed to encode ratchet sessions: %w", err)
	}

	return nil
}

// DeleteRatchetSession deletes a ratchet session
func (s *SessionStorage) DeleteRatchetSession(addr protocol.Address) error {
	sessions, err := s.LoadAllRatchetSessions()
	if err != nil {
		return err
	}

	addrHex := hex.EncodeToString(addr[:])
	delete(sessions, addrHex)

	return s.saveAllRatchetSessions(sessions)
}

// SaveKeyBundleCache saves cached key bundles
func (s *SessionStorage) SaveKeyBundleCache(cache map[protocol.Address]*protocol.KeyBundle) error {
	filePath := filepath.Join(s.storageDir, "key_bundle_cache.json")

	// Convert to string-keyed map for JSON
	stringCache := make(map[string]*protocol.KeyBundle)
	for addr, bundle := range cache {
		addrHex := hex.EncodeToString(addr[:])
		stringCache[addrHex] = bundle
	}

	data, err := json.MarshalIndent(stringCache, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal key bundle cache: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write key bundle cache: %w", err)
	}

	return nil
}

// LoadKeyBundleCache loads cached key bundles
func (s *SessionStorage) LoadKeyBundleCache() (map[protocol.Address]*protocol.KeyBundle, error) {
	filePath := filepath.Join(s.storageDir, "key_bundle_cache.json")

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[protocol.Address]*protocol.KeyBundle), nil
		}
		return nil, fmt.Errorf("failed to read key bundle cache: %w", err)
	}

	var stringCache map[string]*protocol.KeyBundle
	if err := json.Unmarshal(data, &stringCache); err != nil {
		return nil, fmt.Errorf("failed to unmarshal key bundle cache: %w", err)
	}

	// Convert back to Address-keyed map
	cache := make(map[protocol.Address]*protocol.KeyBundle)
	for addrHex, bundle := range stringCache {
		addrBytes, err := hex.DecodeString(addrHex)
		if err != nil {
			continue // Skip invalid entries
		}
		var addr protocol.Address
		copy(addr[:], addrBytes)
		cache[addr] = bundle
	}

	return cache, nil
}

// Clear removes all stored session data
func (s *SessionStorage) Clear() error {
	files := []string{
		"x3dh_state.json",
		"ratchet_sessions.gob",
		"key_bundle_cache.json",
	}

	for _, file := range files {
		filePath := filepath.Join(s.storageDir, file)
		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove %s: %w", file, err)
		}
	}

	return nil
}
