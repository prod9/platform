package srv

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	testAppID        = int64(424242)
	testInstallToken = "ghs_installtoken"
)

// testAppKey generates a throwaway RSA key and returns it with its PKCS#1 PEM form —
// the shape GitHub issues App private keys in.
func testAppKey(t *testing.T) (*rsa.PrivateKey, string) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	return key, string(keyPEM)
}

func TestAppJWT(t *testing.T) {
	key, keyPEM := testAppKey(t)
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)

	jwt, err := appJWT(&GitHubApp{AppID: testAppID, PrivateKey: keyPEM}, now)
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
	_, err := appJWT(&GitHubApp{AppID: 1, PrivateKey: "not a pem"}, time.Now())
	require.Error(t, err)
}

// installationAPIMux handles the two calls minting walks: the repo installation
// lookup (asserting a well-formed App JWT arrives) and the access-token create.
// stubGitHubHooks extends it with the hook-create endpoint.
func installationAPIMux(t *testing.T) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/prod9/app/installation", func(resp http.ResponseWriter, req *http.Request) {
		auth := req.Header.Get("Authorization")
		require.True(t, strings.HasPrefix(auth, "Bearer "))
		require.Len(t, strings.Split(strings.TrimPrefix(auth, "Bearer "), "."), 3)

		resp.Header().Set("Content-Type", "application/json")
		resp.Write([]byte(`{"id": 42}`))
	})
	mux.HandleFunc("POST /app/installations/42/access_tokens", func(resp http.ResponseWriter, req *http.Request) {
		resp.Header().Set("Content-Type", "application/json")
		resp.WriteHeader(http.StatusCreated)
		resp.Write([]byte(`{"token": "` + testInstallToken + `"}`))
	})
	return mux
}

func stubInstallationAPI(t *testing.T) *httptest.Server {
	server := httptest.NewServer(installationAPIMux(t))
	t.Cleanup(server.Close)
	return server
}

func TestMintInstallationToken(t *testing.T) {
	_, keyPEM := testAppKey(t)
	github := stubInstallationAPI(t)

	token, err := mintInstallationToken(t.Context(), github.Client(), github.URL,
		&GitHubApp{AppID: testAppID, PrivateKey: keyPEM}, "prod9", "app")
	require.NoError(t, err)
	require.Equal(t, testInstallToken, token)
}

func TestMintInstallationTokenNotInstalled(t *testing.T) {
	_, keyPEM := testAppKey(t)
	github := httptest.NewServer(http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		resp.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(github.Close)

	_, err := mintInstallationToken(t.Context(), github.Client(), github.URL,
		&GitHubApp{AppID: testAppID, PrivateKey: keyPEM}, "prod9", "app")
	require.ErrorIs(t, err, ErrAppNotInstalled)
	require.ErrorContains(t, err, "prod9/app")
}
