package github

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"time"
)

// jwt mints the App JWT: RS256, iss = client id (GitHub's recommended issuer), iat
// backdated 60s for clock drift, exp 9 minutes out — under GitHub's 10-minute cap
// even when our clock runs ahead. See docs/vendor/github-app-api.md.
func (a *App) jwt(now time.Time) (string, error) {
	block, _ := pem.Decode([]byte(a.PrivateKey))
	if block == nil {
		return "", errors.New("github: app private key is not PEM")
	}
	// GitHub downloads keys as PKCS1 ("RSA PRIVATE KEY"), but a key re-encoded in
	// transit (openssl, some secret stores) arrives as PKCS8 — accept both.
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		parsed, pkcs8Err := x509.ParsePKCS8PrivateKey(block.Bytes)
		rsaKey, isRSA := parsed.(*rsa.PrivateKey)
		if pkcs8Err != nil || !isRSA {
			return "", fmt.Errorf("github: parsing app private key (PKCS1 or PKCS8 RSA): %w", err)
		}
		key = rsaKey
	}

	claims, err := json.Marshal(struct {
		Iss string `json:"iss"`
		Iat int64  `json:"iat"`
		Exp int64  `json:"exp"`
	}{a.ClientID, now.Add(-time.Minute).Unix(), now.Add(9 * time.Minute).Unix()})
	if err != nil {
		return "", fmt.Errorf("github: encoding jwt claims: %w", err)
	}

	b64 := base64.RawURLEncoding
	signing := b64.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`)) +
		"." + b64.EncodeToString(claims)
	digest := sha256.Sum256([]byte(signing))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("github: signing jwt: %w", err)
	}

	return signing + "." + b64.EncodeToString(sig), nil
}
