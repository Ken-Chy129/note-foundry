package identity

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

func GenerateSecureToken() (string, error) {
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("read secure random bytes: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(randomBytes), nil
}
