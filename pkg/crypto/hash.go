package crypto

import (
	"crypto/rand"
	"encoding/hex"

	"golang.org/x/crypto/blake2b"
)

// Hash generates a BLAKE2b-256 hash
func Hash(data []byte) ([]byte, error) {
	hash, err := blake2b.New256(nil)
	if err != nil {
		return nil, err
	}

	hash.Write(data)
	return hash.Sum(nil), nil
}

// HashString generates a BLAKE2b hash and returns hex string
func HashString(data []byte) (string, error) {
	hash, err := Hash(data)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hash), nil
}

// GenerateNonce generates a random nonce
func GenerateNonce(size int) ([]byte, error) {
	nonce := make([]byte, size)
	_, err := rand.Read(nonce)
	if err != nil {
		return nil, err
	}
	return nonce, nil
}

// VerifyHash verifies a hash matches the data
func VerifyHash(data []byte, expectedHash []byte) (bool, error) {
	actualHash, err := Hash(data)
	if err != nil {
		return false, err
	}

	if len(actualHash) != len(expectedHash) {
		return false, nil
	}

	for i := range actualHash {
		if actualHash[i] != expectedHash[i] {
			return false, nil
		}
	}

	return true, nil
}
