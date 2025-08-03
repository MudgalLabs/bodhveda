package entity

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/mudgallabs/bodhveda/internal/env"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/tantra/cipher"
)

type APIKey struct {
	ID        int
	Name      string
	Token     []byte // Encrypted token.
	Nonce     []byte // Nonce used for encryption.
	TokenHash string // HMAC-SHA256 hash of the token, used for DB lookup.
	Scope     enum.APIKeyScope
	ProjectID int
	UserID    int
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewAPIKey(userID, projectID int, name string, scope enum.APIKeyScope) (*APIKey, error) {
	now := time.Now().UTC()

	tokenPlain, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	token, nonce, err := cipher.Encrypt([]byte(tokenPlain), []byte(env.CipherKey))
	if err != nil {
		return nil, fmt.Errorf("encrypt token: %w", err)
	}

	tokenHash := cipher.HashToken(tokenPlain, []byte(env.HashKey))

	return &APIKey{
		Name:      name,
		UserID:    userID,
		ProjectID: projectID,
		Token:     token,
		Nonce:     nonce,
		TokenHash: tokenHash,
		Scope:     scope,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func generateToken() (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	token := make([]byte, 32)
	for i := range token {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		token[i] = charset[num.Int64()]
	}

	return "bv_" + string(token), nil
}
