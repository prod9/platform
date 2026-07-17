package github

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/srv/srvtest"
)

const testAppID = int64(424242)

func TestAppJWT(t *testing.T) {
	key, keyPEM := srvtest.AppKey(t)
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)

	jwt, err := appJWT(&App{AppID: testAppID, PrivateKey: keyPEM}, now)
	require.NoError(t, err)
	parts := strings.Split(jwt, ".")
	require.Len(t, parts, 3)

	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	require.NoError(t, err)
	header := map[string]string{}
	require.NoError(t, json.Unmarshal(headerJSON, &header))
	require.Equal(t, map[string]string{"alg": "RS256", "typ": "JWT"}, header)

	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var claims struct {
		Iat int64  `json:"iat"`
		Exp int64  `json:"exp"`
		Iss string `json:"iss"`
	}
	require.NoError(t, json.Unmarshal(claimsJSON, &claims))
	require.Equal(t, now.Add(-time.Minute).Unix(), claims.Iat)
	require.Equal(t, now.Add(9*time.Minute).Unix(), claims.Exp)
	require.Equal(t, "424242", claims.Iss)

	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	require.NoError(t, err)
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	require.NoError(t, rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, digest[:], signature))
}

func TestAppJWTRejectsBadKey(t *testing.T) {
	_, err := appJWT(&App{AppID: 1, PrivateKey: "not a pem"}, time.Now())
	require.Error(t, err)
}

func stubInstallationAPI(t *testing.T) *httptest.Server {
	server := httptest.NewServer(srvtest.InstallationAPIMux(t))
	t.Cleanup(server.Close)
	return server
}

func TestMintInstallationToken(t *testing.T) {
	_, keyPEM := srvtest.AppKey(t)
	github := stubInstallationAPI(t)

	token, err := MintInstallationToken(t.Context(), github.Client(), github.URL,
		&App{AppID: testAppID, PrivateKey: keyPEM}, "prod9", "app")
	require.NoError(t, err)
	require.Equal(t, srvtest.InstallToken, token)
}

func TestMintInstallationTokenNotInstalled(t *testing.T) {
	_, keyPEM := srvtest.AppKey(t)
	github := httptest.NewServer(http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		resp.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(github.Close)

	_, err := MintInstallationToken(t.Context(), github.Client(), github.URL,
		&App{AppID: testAppID, PrivateKey: keyPEM}, "prod9", "app")
	require.ErrorIs(t, err, ErrAppNotInstalled)
	require.ErrorContains(t, err, "prod9/app")
}
