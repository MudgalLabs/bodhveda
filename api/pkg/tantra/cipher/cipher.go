package cipher

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
)

// Encrypt encrypts plaintext using AES-256-GCM and returns ciphertext and nonce
func Encrypt(plaintext, secret []byte) (ciphertext []byte, nonce []byte, err error) {
	block, err := aes.NewCipher(secret)
	if err != nil {
		return nil, nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}

	nonce = make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}

	ciphertext = aesGCM.Seal(nil, nonce, plaintext, nil)
	return ciphertext, nonce, nil
}

// Decrypt decrypts ciphertext using AES-256-GCM with the given secret and nonce
func Decrypt(ciphertext, nonce, secret []byte) (string, error) {
	if len(nonce) != 12 {
		return "", errors.New("nonce length must be 12 bytes")
	}

	block, err := aes.NewCipher(secret)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	plaintextStr := string(plaintext)
	return plaintextStr, nil
}

// HashToken returns a HMAC-SHA256 hash of the token.
func HashToken(token string, secret []byte) string {
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(token))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}
