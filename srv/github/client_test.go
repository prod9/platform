package github

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/fxtest"
	"github.com/stretchr/testify/require"
)

func testClient(t *testing.T, handler http.Handler) (*Client, *rsa.PrivateKey) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	t.Setenv("GITHUB_APP_ID", "42")
	t.Setenv("GITHUB_APP_PRIVATE_KEY", string(keyPEM))
	t.Setenv("GITHUB_APP_WEBHOOK_SECRET", "whsec")
	t.Setenv("GITHUB_APP_CLIENT_ID", "Iv1.abc")
	t.Setenv("GITHUB_APP_CLIENT_SECRET", "csec")
	t.Setenv("GITHUB_API_URL", server.URL)
	ctx := config.NewContext(context.Background(), fxtest.Configure())

	client, err := NewClient(ctx)
	require.NoError(t, err)
	return client, key
}

// requireAppJWT verifies the Authorization header carries a valid RS256 App JWT:
// signature by our key, iss = client id, iat backdated, exp within 10 minutes.
func requireAppJWT(t *testing.T, req *http.Request, key *rsa.PrivateKey) {
	auth := req.Header.Get("Authorization")
	require.True(t, strings.HasPrefix(auth, "Bearer "), "want Bearer JWT, got %q", auth)

	parts := strings.Split(strings.TrimPrefix(auth, "Bearer "), ".")
	require.Len(t, parts, 3)

	signed := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	require.NoError(t, err)
	require.NoError(t, rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, signed[:], sig))

	var header struct {
		Alg string `json:"alg"`
	}
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(headerJSON, &header))
	require.Equal(t, "RS256", header.Alg)

	var claims struct {
		Iss string `json:"iss"`
		Iat int64  `json:"iat"`
		Exp int64  `json:"exp"`
	}
	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(claimsJSON, &claims))

	now := time.Now().Unix()
	require.Equal(t, "Iv1.abc", claims.Iss)
	require.LessOrEqual(t, claims.Iat, now-50, "iat must be backdated for clock drift")
	require.Greater(t, claims.Exp, now)
	require.LessOrEqual(t, claims.Exp, now+10*60)
}

func TestInstallationToken(t *testing.T) {
	var client *Client
	var key *rsa.PrivateKey
	client, key = testClient(t, http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		require.Equal(t, "POST", req.Method)
		require.Equal(t, "/app/installations/7/access_tokens", req.URL.Path)
		requireAppJWT(t, req, key)

		resp.WriteHeader(201)
		fmt.Fprint(resp, `{"token":"ghs_tok","expires_at":"2026-08-03T12:00:00Z"}`)
	}))

	token, err := client.InstallationToken(context.Background(), 7)
	require.NoError(t, err)
	require.Equal(t, "ghs_tok", token)
}

func TestInstallationTokenAPIError(t *testing.T) {
	client, _ := testClient(t, http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		resp.WriteHeader(500)
		fmt.Fprint(resp, `{"message":"boom"}`)
	}))

	_, err := client.InstallationToken(context.Background(), 7)
	require.ErrorContains(t, err, "500")
	require.ErrorContains(t, err, "boom")
}

func TestInstallationOrg(t *testing.T) {
	var client *Client
	var key *rsa.PrivateKey
	client, key = testClient(t, http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		require.Equal(t, "GET", req.Method)
		require.Equal(t, "/app/installations/7", req.URL.Path)
		requireAppJWT(t, req, key)

		fmt.Fprint(resp, `{"id":7,"account":{"login":"prodigy9","type":"Organization"}}`)
	}))

	org, err := client.InstallationOrg(context.Background(), 7)
	require.NoError(t, err)
	require.Equal(t, "prodigy9", org)
}

func TestIsOrgOwner(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		owner  bool
	}{
		{"active admin is owner", 200, `{"role":"admin","state":"active"}`, true},
		{"member is not owner", 200, `{"role":"member","state":"active"}`, false},
		{"pending admin is not owner", 200, `{"role":"admin","state":"pending"}`, false},
		{"non-member is not owner", 404, `{"message":"Not Found"}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client, _ := testClient(t, http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
				require.Equal(t, "GET", req.Method)
				require.Equal(t, "/orgs/prodigy9/memberships/chakrit", req.URL.Path)
				require.Equal(t, "Bearer ghs_tok", req.Header.Get("Authorization"))

				resp.WriteHeader(tc.status)
				fmt.Fprint(resp, tc.body)
			}))

			owner, err := client.IsOrgOwner(context.Background(), "ghs_tok", "prodigy9", "chakrit")
			require.NoError(t, err)
			require.Equal(t, tc.owner, owner)
		})
	}
}

func TestRepos(t *testing.T) {
	client, _ := testClient(t, http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		require.Equal(t, "GET", req.Method)
		require.Equal(t, "/installation/repositories", req.URL.Path)
		require.Equal(t, "100", req.URL.Query().Get("per_page"))
		require.Equal(t, "Bearer ghs_tok", req.Header.Get("Authorization"))

		var repos []string
		switch req.URL.Query().Get("page") {
		case "", "1":
			for i := range 100 {
				repos = append(repos, fmt.Sprintf(
					`{"name":"repo%d","full_name":"prodigy9/repo%d","owner":{"login":"prodigy9"}}`, i, i))
			}
		case "2":
			repos = []string{`{"name":"last","full_name":"prodigy9/last","owner":{"login":"prodigy9"}}`}
		}
		fmt.Fprintf(resp, `{"total_count":101,"repositories":[%s]}`, strings.Join(repos, ","))
	}))

	repos, err := client.Repos(context.Background(), "ghs_tok")
	require.NoError(t, err)
	require.Len(t, repos, 101)
	require.Equal(t, Repo{Name: "repo0", FullName: "prodigy9/repo0", Owner: "prodigy9"}, repos[0])
	require.Equal(t, Repo{Name: "last", FullName: "prodigy9/last", Owner: "prodigy9"}, repos[100])
}

func TestResolveRef(t *testing.T) {
	client, _ := testClient(t, http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		require.Equal(t, "GET", req.Method)
		require.Equal(t, "/repos/prodigy9/platform/commits/tags/v1.2.3", req.URL.Path)
		require.Equal(t, "Bearer ghs_tok", req.Header.Get("Authorization"))
		require.Equal(t, "application/vnd.github.sha", req.Header.Get("Accept"))

		fmt.Fprint(resp, "e4c7a1")
	}))

	sha, err := client.ResolveRef(context.Background(), "ghs_tok", "prodigy9", "platform", "tags/v1.2.3")
	require.NoError(t, err)
	require.Equal(t, "e4c7a1", sha)
}

func TestResolveRefRejectsBadRepoPath(t *testing.T) {
	client, _ := testClient(t, http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		t.Error("request must not reach the API")
	}))

	_, err := client.ResolveRef(context.Background(), "ghs_tok", "../etc", "platform", "v1")
	require.ErrorContains(t, err, "invalid repo path")
}
