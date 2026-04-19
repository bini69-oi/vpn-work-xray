package sqlite

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"os"
	"strings"
)

func subscriptionTokenSecretKey() ([]byte, bool) {
	raw := strings.TrimSpace(os.Getenv("VPN_PRODUCT_SUBSCRIPTION_TOKEN_KEY"))
	if raw == "" {
		return nil, false
	}
	sum := sha256.Sum256([]byte(raw))
	return sum[:], true
}

func sealSubscriptionToken(plain string) (string, error) {
	if strings.TrimSpace(plain) == "" {
		return "", errors.New("empty token")
	}
	key, ok := subscriptionTokenSecretKey()
	if !ok {
		return "", errors.New("subscription token key not configured")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	out := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(out), nil
}

func openSubscriptionToken(enc string) (string, error) {
	enc = strings.TrimSpace(enc)
	if enc == "" {
		return "", errors.New("empty ciphertext")
	}
	key, ok := subscriptionTokenSecretKey()
	if !ok {
		return "", errors.New("subscription token key not configured")
	}
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}
	nonce := raw[:gcm.NonceSize()]
	ciphertext := raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
