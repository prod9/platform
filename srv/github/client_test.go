package github

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
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

	// Credentials live in settings now; stub the loader rather than dragging a
	// database into every client test — the settings-backed read has its own tests
	// in app_test.go.
	app := &App{AppID: 42, PrivateKey: string(keyPEM),
		WebhookSecret: "whsec", ClientID: "Iv1.abc", ClientSecret: "csec"}
	orig := LoadApp
	LoadApp = func(context.Context) (*App, error) { return app, nil }
	t.Cleanup(func() { LoadApp = orig })

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

func TestJWTAcceptsPKCS8Key(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})

	app := &App{ClientID: "Iv1.abc", PrivateKey: string(keyPEM)}
	token, err := app.jwt(time.Now())
	require.NoError(t, err)
	require.NotEmpty(t, token)
}

// A PKCS8 key that parses but is not RSA (e.g. EC) must say so — the old wrap
// surfaced only the PKCS1 error, misdiagnosing a wrong-algorithm key as a bad encoding.
func TestJWTRejectsNonRSAPKCS8Key(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})

	app := &App{ClientID: "Iv1.abc", PrivateKey: string(keyPEM)}
	_, err = app.jwt(time.Now())
	require.ErrorContains(t, err, "not RSA")
}

// A PEM block that is neither PKCS1 nor PKCS8 reports both parse failures — either
// alone would point the operator at the wrong half of the fallback.
func TestJWTBadKeyReportsBothEncodings(t *testing.T) {
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("garbage")})

	app := &App{ClientID: "Iv1.abc", PrivateKey: string(keyPEM)}
	_, err := app.jwt(time.Now())
	require.ErrorContains(t, err, "PKCS1:")
	require.ErrorContains(t, err, "PKCS8:")
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

		fmt.Fprint(resp, `{"id":7,"account":{"id":9,"login":"prodigy9","type":"Organization"}}`)
	}))

	org, err := client.InstallationOrg(context.Background(), 7)
	require.NoError(t, err)
	require.Equal(t, &Org{ID: 9, Login: "prodigy9"}, org)
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

func TestReposNeverEndingPagesIsAnError(t *testing.T) {
	client, _ := testClient(t, http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		var repos []string
		for i := range 100 {
			repos = append(repos, fmt.Sprintf(
				`{"name":"repo%d","full_name":"prodigy9/repo%d","owner":{"login":"prodigy9"}}`, i, i))
		}
		fmt.Fprintf(resp, `{"total_count":10000,"repositories":[%s]}`, strings.Join(repos, ","))
	}))

	_, err := client.Repos(context.Background(), "ghs_tok")
	require.ErrorContains(t, err, "did not end")
}

func TestRepoCloneURL(t *testing.T) {
	client, _ := testClient(t, http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		require.Equal(t, "GET", req.Method)
		require.Equal(t, "/repos/prodigy9/platform", req.URL.Path)
		require.Equal(t, "Bearer ghs_tok", req.Header.Get("Authorization"))

		fmt.Fprint(resp, `{"clone_url":"https://github.com/prodigy9/platform.git"}`)
	}))

	cloneURL, err := client.RepoCloneURL(context.Background(), "ghs_tok", "prodigy9", "platform")
	require.NoError(t, err)
	require.Equal(t, "https://github.com/prodigy9/platform.git", cloneURL)
}

func TestRepoCloneURLUnreachableRepo(t *testing.T) {
	client, _ := testClient(t, http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		resp.WriteHeader(404)
	}))

	_, err := client.RepoCloneURL(context.Background(), "ghs_tok", "prodigy9", "hidden")
	require.ErrorIs(t, err, ErrRepoUnreachable)
}

func TestRepoCloneURLRejectsBadRepoPath(t *testing.T) {
	client, _ := testClient(t, http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		t.Error("request must not reach the API")
	}))

	_, err := client.RepoCloneURL(context.Background(), "ghs_tok", "..", "platform")
	require.Error(t, err)
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

func TestResolveRefUnresolvable(t *testing.T) {
	for _, status := range []int{404, 422} {
		client, _ := testClient(t, http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			resp.WriteHeader(status)
		}))

		_, err := client.ResolveRef(context.Background(), "ghs_tok", "prodigy9", "platform", "tags/nope")
		require.ErrorIs(t, err, ErrRefUnresolvable, "status=%d", status)
	}
}

func TestResolveRefRejectsBadRepoPath(t *testing.T) {
	client, _ := testClient(t, http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		t.Error("request must not reach the API")
	}))

	_, err := client.ResolveRef(context.Background(), "ghs_tok", "../etc", "platform", "v1")
	require.ErrorContains(t, err, "invalid repo path")
}

func TestAppPermissions(t *testing.T) {
	var seen *http.Request
	client, key := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Clone(context.Background())
		require.Equal(t, "/app", r.URL.Path)
		fmt.Fprint(w, `{"permissions":{"contents":"write","metadata":"read","members":"read","organization_hooks":"write"}}`)
	}))

	perms, err := client.AppPermissions(context.Background())
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"contents": "write", "metadata": "read",
		"members": "read", "organization_hooks": "write",
	}, perms)
	requireAppJWT(t, seen, key)
}

func TestMissingPermissions(t *testing.T) {
	full := map[string]string{
		"contents": "write", "metadata": "read",
		"members": "read", "organization_hooks": "write",
	}
	require.Empty(t, MissingPermissions(full))

	// write satisfies a read requirement
	elevated := map[string]string{
		"contents": "write", "metadata": "write",
		"members": "write", "organization_hooks": "write",
	}
	require.Empty(t, MissingPermissions(elevated))

	// a read where write is required, and an absent slug, are both named
	underscoped := map[string]string{
		"contents": "read", "metadata": "read",
	}
	require.Equal(t,
		[]string{"contents: write", "members: read"},
		MissingPermissions(underscoped))
}
