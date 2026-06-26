package credentials

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

const AlgorithmAESGCM = "aes-256-gcm"

const aes256KeyLength = 32

//nolint:gochecknoglobals // Narrow package seams keep crypto failure paths testable.
var (
	randomReader = rand.Reader
	newAESCipher = aes.NewCipher
	newGCM       = cipher.NewGCM
)

type Envelope struct {
	KeyVersion string
	Algorithm  string
	Nonce      string
	Ciphertext string
}

type AESGCMCipher struct {
	aead       cipher.AEAD
	keyVersion string
}

func NewAESGCMCipher(key []byte, keyVersion string) (*AESGCMCipher, error) {
	trimmedVersion := strings.TrimSpace(keyVersion)
	if len(key) != aes256KeyLength {
		return nil, fmt.Errorf("aes-gcm key must be 32 bytes, got %d", len(key))
	}
	if trimmedVersion == "" {
		return nil, errors.New("key version is required")
	}
	block, err := newAESCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create aes cipher: %w", err)
	}
	aead, err := newGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm cipher: %w", err)
	}
	return &AESGCMCipher{aead: aead, keyVersion: trimmedVersion}, nil
}

func (c *AESGCMCipher) SealString(plaintext string) (Envelope, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(randomReader, nonce); err != nil {
		return Envelope{}, fmt.Errorf("read nonce: %w", err)
	}
	ciphertext := c.aead.Seal(nil, nonce, []byte(plaintext), nil)
	return Envelope{
		KeyVersion: c.keyVersion,
		Algorithm:  AlgorithmAESGCM,
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
	}, nil
}

func (c *AESGCMCipher) OpenString(envelope Envelope) (string, error) {
	if envelope.Algorithm != AlgorithmAESGCM {
		return "", fmt.Errorf("unsupported algorithm %q", envelope.Algorithm)
	}
	nonce, err := base64.StdEncoding.DecodeString(envelope.Nonce)
	if err != nil {
		return "", fmt.Errorf("decode nonce: %w", err)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(envelope.Ciphertext)
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}
	plaintext, err := c.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt ciphertext: %w", err)
	}
	return string(plaintext), nil
}
