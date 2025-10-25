package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
)

var (
	ErrInvalidKey     = errors.New("invalid key")
	ErrEncryptionFailed = errors.New("encryption failed")
	ErrDecryptionFailed = errors.New("decryption failed")
)

// GenerateRSAKeyPair generates a new RSA-4096 key pair
func GenerateRSAKeyPair() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, 4096)
}

// ExportPrivateKeyPEM exports private key to PEM format
func ExportPrivateKeyPEM(key *rsa.PrivateKey) ([]byte, error) {
	privASN1 := x509.MarshalPKCS1PrivateKey(key)

	privBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privASN1,
	}

	return pem.EncodeToMemory(privBlock), nil
}

// ExportPublicKeyPEM exports public key to PEM format
func ExportPublicKeyPEM(key *rsa.PublicKey) ([]byte, error) {
	pubASN1, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return nil, err
	}

	pubBlock := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubASN1,
	}

	return pem.EncodeToMemory(pubBlock), nil
}

// ImportPrivateKeyPEM imports private key from PEM format
func ImportPrivateKeyPEM(pemData []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, ErrInvalidKey
	}

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	return key, nil
}

// ImportPublicKeyPEM imports public key from PEM format
func ImportPublicKeyPEM(pemData []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, ErrInvalidKey
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, ErrInvalidKey
	}

	return rsaPub, nil
}

// SaveKeyToFile saves a PEM encoded key to file
func SaveKeyToFile(filename string, pemData []byte) error {
	return os.WriteFile(filename, pemData, 0600)
}

// LoadKeyFromFile loads a PEM encoded key from file
func LoadKeyFromFile(filename string) ([]byte, error) {
	return os.ReadFile(filename)
}

// RSAEncrypt encrypts data with RSA public key using OAEP
func RSAEncrypt(data []byte, publicKey *rsa.PublicKey) ([]byte, error) {
	hash := sha256.New()
	ciphertext, err := rsa.EncryptOAEP(hash, rand.Reader, publicKey, data, nil)
	if err != nil {
		return nil, ErrEncryptionFailed
	}
	return ciphertext, nil
}

// RSADecrypt decrypts data with RSA private key using OAEP
func RSADecrypt(ciphertext []byte, privateKey *rsa.PrivateKey) ([]byte, error) {
	hash := sha256.New()
	plaintext, err := rsa.DecryptOAEP(hash, rand.Reader, privateKey, ciphertext, nil)
	if err != nil {
		return nil, ErrDecryptionFailed
	}
	return plaintext, nil
}

// SignData signs data with RSA private key
func SignData(data []byte, privateKey *rsa.PrivateKey) ([]byte, error) {
	hash := sha256.New()
	hash.Write(data)
	hashed := hash.Sum(nil)

	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, 0, hashed)
	if err != nil {
		return nil, err
	}

	return signature, nil
}

// VerifySignature verifies signature with RSA public key
func VerifySignature(data []byte, signature []byte, publicKey *rsa.PublicKey) error {
	hash := sha256.New()
	hash.Write(data)
	hashed := hash.Sum(nil)

	return rsa.VerifyPKCS1v15(publicKey, 0, hashed, signature)
}
