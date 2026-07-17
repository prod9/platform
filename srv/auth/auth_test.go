package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/data/migrator"
	"fx.prodigy9.co/fxtest"
	"fx.prodigy9.co/httpserver/middlewares"
	"fx.prodigy9.co/secret"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/srv/github"
	"platform.prodigy9.co/srv/srvtest"
)

const (
	testServerURL    = "https://platform.example.com"
	testClientID     = "Iv1.abc"
	testClientSecret = "csec"
	testAccessToken  = "gho_usertoken"
)

func setupDB(t *testing.T) context.Context {
	return srvtest.SetupDB(t, migrator.FromFS(Migrations))
}

func stubApp(t *testing.T, app *github.App, err error) {
	orig := github.LoadApp
	github.LoadApp = func(ctx context.Context) (*github.App, error) { return app, err }
	t.Cleanup(func() { github.LoadApp = orig })
}

func authRouter(t *testing.T, cfg *config.Source) chi.Router {
	config.Set(cfg, github.ServerURLConfig, testServerURL)
	router := chi.NewRouter()
	router.Use(middlewares.Configure(cfg))
	require.NoError(t, SessionCtr{}.Mount(cfg, router))
	return router
}

// startTestSession seeds a user with a live (or expired) session, returning the user
// id and the raw session token the client-side cookie would carry.
func startTestSession(t *testing.T, ctx context.Context, expiresAt time.Time) (int64, string) {
	var userID int64
	require.NoError(t, data.Get(ctx, &userID,
		`INSERT INTO users (name) VALUES ('octocat') RETURNING id`))

	token := randomToken()
	create := &CreateSession{UserID: userID, Token: token, ExpiresAt: expiresAt}
	require.NoError(t, create.Execute(ctx, nil))
	return userID, token
}

func usersMeRequest(ctx context.Context, token string) *http.Request {
	req := httptest.NewRequest("GET", "/api/users/me", nil).WithContext(ctx)
	if token != "" {
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
	}
	return req
}

func sessionRequest(ctx context.Context, token string) *http.Request {
	req := httptest.NewRequest("GET", "/api/session", nil).WithContext(ctx)
	if token != "" {
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
	}
	return req
}

func responseCookie(t *testing.T, resp *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, cookie := range resp.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("no %s cookie in response", name)
	return nil
}

func TestUsersMeWithoutCookie(t *testing.T) {
	router := authRouter(t, fxtest.Configure())

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest("GET", "/api/users/me", nil))

	require.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestUsersMeWithSession(t *testing.T) {
	ctx := setupDB(t)
	userID, token := startTestSession(t, ctx, time.Now().Add(time.Hour))
	router := authRouter(t, fxtest.Configure())

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, usersMeRequest(ctx, token))

	require.Equal(t, http.StatusOK, resp.Code)

	var body struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	require.Equal(t, userID, body.ID)
	require.Equal(t, "octocat", body.Name)
}

func TestUsersMeWithExpiredSession(t *testing.T) {
	ctx := setupDB(t)
	_, token := startTestSession(t, ctx, time.Now().Add(-time.Hour))
	router := authRouter(t, fxtest.Configure())

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, usersMeRequest(ctx, token))

	require.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestSessionWithoutCookie(t *testing.T) {
	router := authRouter(t, fxtest.Configure())

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest("GET", "/api/session", nil))

	require.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestSessionWithLiveSession(t *testing.T) {
	ctx := setupDB(t)
	expiresAt := time.Now().Add(time.Hour)
	userID, token := startTestSession(t, ctx, expiresAt)
	router := authRouter(t, fxtest.Configure())

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, sessionRequest(ctx, token))

	require.Equal(t, http.StatusOK, resp.Code)

	var body struct {
		UserID    int64     `json:"user_id"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	require.Equal(t, userID, body.UserID)
	require.WithinDuration(t, expiresAt, body.ExpiresAt, time.Second)
}

func TestSessionWithExpiredSession(t *testing.T) {
	ctx := setupDB(t)
	_, token := startTestSession(t, ctx, time.Now().Add(-time.Hour))
	router := authRouter(t, fxtest.Configure())

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, sessionRequest(ctx, token))

	require.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestGitHubLoginRedirectsToAuthorize(t *testing.T) {
	stubApp(t, &github.App{ClientID: testClientID}, nil)
	router := authRouter(t, fxtest.Configure())

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest("GET", "/auth/github", nil))

	require.Equal(t, http.StatusTemporaryRedirect, resp.Code)

	state := responseCookie(t, resp, oauthStateCookie)
	require.NotEmpty(t, state.Value)
	require.True(t, state.HttpOnly)
	require.True(t, state.Secure)
	require.Equal(t, http.SameSiteLaxMode, state.SameSite)
	require.Equal(t, int((10 * time.Minute).Seconds()), state.MaxAge)

	location, err := url.Parse(resp.Header().Get("Location"))
	require.NoError(t, err)
	require.Equal(t, "https://github.com/login/oauth/authorize", location.Scheme+"://"+location.Host+location.Path)
	require.Equal(t, testClientID, location.Query().Get("client_id"))
	require.Equal(t, testServerURL+"/auth/github/callback", location.Query().Get("redirect_uri"))
	require.Equal(t, state.Value, location.Query().Get("state"))
}

func TestGitHubLoginWithoutApp(t *testing.T) {
	stubApp(t, nil, github.ErrNoApp)
	router := authRouter(t, fxtest.Configure())

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest("GET", "/auth/github", nil))

	require.Equal(t, http.StatusServiceUnavailable, resp.Code)
}

func TestGitHubCallbackStateMismatch(t *testing.T) {
	stubApp(t, &github.App{ClientID: testClientID, ClientSecret: testClientSecret}, nil)
	router := authRouter(t, fxtest.Configure())

	missingCookie := httptest.NewRequest("GET", "/auth/github/callback?code=C&state=abc", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, missingCookie)
	require.Equal(t, http.StatusBadRequest, resp.Code)

	mismatched := httptest.NewRequest("GET", "/auth/github/callback?code=C&state=abc", nil)
	mismatched.AddCookie(&http.Cookie{Name: oauthStateCookie, Value: "xyz"})
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, mismatched)
	require.Equal(t, http.StatusBadRequest, resp.Code)

	missingState := httptest.NewRequest("GET", "/auth/github/callback?code=C", nil)
	missingState.AddCookie(&http.Cookie{Name: oauthStateCookie, Value: ""})
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, missingState)
	require.Equal(t, http.StatusBadRequest, resp.Code)
}

func TestExchangeOAuthCode(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		require.Equal(t, "POST", req.Method)
		require.Equal(t, "/login/oauth/access_token", req.URL.Path)
		require.Equal(t, "application/json", req.Header.Get("Accept"))

		require.NoError(t, req.ParseForm())
		require.Equal(t, testClientID, req.PostForm.Get("client_id"))
		require.Equal(t, testClientSecret, req.PostForm.Get("client_secret"))
		require.Equal(t, "CODE123", req.PostForm.Get("code"))

		resp.Header().Set("Content-Type", "application/json")
		resp.Write([]byte(`{"access_token": "` + testAccessToken + `", "token_type": "bearer"}`))
	}))
	defer stub.Close()

	token, err := exchangeOAuthCode(t.Context(), stub.Client(), stub.URL,
		testClientID, testClientSecret, "CODE123")
	require.NoError(t, err)
	require.Equal(t, testAccessToken, token)
}

func TestExchangeOAuthCodeRejectsErrorResponse(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		resp.Header().Set("Content-Type", "application/json")
		resp.Write([]byte(`{"error": "bad_verification_code", "error_description": "expired"}`))
	}))
	defer stub.Close()

	_, err := exchangeOAuthCode(t.Context(), stub.Client(), stub.URL,
		testClientID, testClientSecret, "EXPIRED")
	require.ErrorContains(t, err, "bad_verification_code")
}

func TestFetchGitHubUser(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		require.Equal(t, "GET", req.Method)
		require.Equal(t, "/user", req.URL.Path)
		require.Equal(t, "Bearer "+testAccessToken, req.Header.Get("Authorization"))

		resp.Header().Set("Content-Type", "application/json")
		resp.Write([]byte(`{"id": 12345, "login": "octocat", "email": "octo@example.com"}`))
	}))
	defer stub.Close()

	account, err := fetchGitHubUser(t.Context(), stub.Client(), stub.URL, testAccessToken)
	require.NoError(t, err)
	require.Equal(t, &githubAccount{ID: 12345, Login: "octocat", Email: "octo@example.com"}, account)
}

func TestFetchGitHubUserHiddenEmail(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		resp.Header().Set("Content-Type", "application/json")
		resp.Write([]byte(`{"id": 12345, "login": "octocat", "email": null}`))
	}))
	defer stub.Close()

	account, err := fetchGitHubUser(t.Context(), stub.Client(), stub.URL, testAccessToken)
	require.NoError(t, err)
	require.Equal(t, "", account.Email)
}

// stubGitHubOAuth serves both OAuth endpoints the callback path hits: the code
// exchange and GET /user.
func stubGitHubOAuth(t *testing.T) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/login/oauth/access_token", func(resp http.ResponseWriter, req *http.Request) {
		resp.Header().Set("Content-Type", "application/json")
		resp.Write([]byte(`{"access_token": "` + testAccessToken + `", "token_type": "bearer"}`))
	})
	mux.HandleFunc("/user", func(resp http.ResponseWriter, req *http.Request) {
		resp.Header().Set("Content-Type", "application/json")
		resp.Write([]byte(`{"id": 12345, "login": "octocat", "email": "octo@example.com"}`))
	})

	stub := httptest.NewServer(mux)
	t.Cleanup(stub.Close)
	return stub
}

func loginCallback(t *testing.T, router chi.Router, ctx context.Context) *http.Cookie {
	req := httptest.NewRequest("GET", "/auth/github/callback?code=C&state=S", nil)
	req.AddCookie(&http.Cookie{Name: oauthStateCookie, Value: "S"})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req.WithContext(ctx))

	require.Equal(t, http.StatusTemporaryRedirect, resp.Code)
	require.Equal(t, "/", resp.Header().Get("Location"))

	state := responseCookie(t, resp, oauthStateCookie)
	require.Empty(t, state.Value, "a successful callback must clear the state cookie")
	require.Negative(t, state.MaxAge)

	return responseCookie(t, resp, sessionCookie)
}

func TestGitHubCallbackCreatesUserIdentityAndSession(t *testing.T) {
	ctx := setupDB(t)
	stubApp(t, &github.App{ClientID: testClientID, ClientSecret: testClientSecret}, nil)
	stub := stubGitHubOAuth(t)

	cfg := fxtest.Configure()
	config.Set(cfg, github.URLConfig, stub.URL)
	config.Set(cfg, github.APIURLConfig, stub.URL)
	router := authRouter(t, cfg)

	session := loginCallback(t, router, ctx)
	require.NotEmpty(t, session.Value)
	require.True(t, session.HttpOnly)
	require.True(t, session.Secure)
	require.Equal(t, http.SameSiteLaxMode, session.SameSite)
	require.Equal(t, "/", session.Path)
	require.Equal(t, int((30 * 24 * time.Hour).Seconds()), session.MaxAge)

	user := &User{}
	require.NoError(t, data.Get(ctx, user, `SELECT * FROM users`))
	require.Equal(t, "octocat", user.Name)

	var identity struct {
		UserID        int64  `db:"user_id"`
		Provider      string `db:"provider"`
		ProviderID    string `db:"provider_id"`
		Kind          string `db:"kind"`
		Email         string `db:"email"`
		EmailVerified bool   `db:"email_verified"`
		Metadata      string `db:"metadata"`
	}
	require.NoError(t, data.Get(ctx, &identity, `
		SELECT user_id, provider, provider_id, kind, email, email_verified,
			metadata::text AS metadata
		FROM identities`))
	require.Equal(t, user.ID, identity.UserID)
	require.Equal(t, "github", identity.Provider)
	require.Equal(t, "12345", identity.ProviderID)
	require.Equal(t, "login", identity.Kind)
	require.Equal(t, "octo@example.com", identity.Email)
	require.False(t, identity.EmailVerified)

	// the user token must be encrypted at rest: the raw jsonb never carries the
	// plaintext, and the stored ciphertext reveals back to it.
	require.NotContains(t, identity.Metadata, testAccessToken)
	metadata := map[string]string{}
	require.NoError(t, json.Unmarshal([]byte(identity.Metadata), &metadata))
	require.Equal(t, "octocat", metadata["login"])
	revealed, err := secret.Reveal(config.FromContext(ctx), metadata["token"])
	require.NoError(t, err)
	require.Equal(t, testAccessToken, revealed)

	var storedSession struct {
		UserID    int64     `db:"user_id"`
		TokenHash string    `db:"token_hash"`
		ExpiresAt time.Time `db:"expires_at"`
	}
	require.NoError(t, data.Get(ctx, &storedSession,
		`SELECT user_id, token_hash, expires_at FROM sessions`))
	require.Equal(t, user.ID, storedSession.UserID)
	require.Equal(t, hashSessionToken(session.Value), storedSession.TokenHash)
	require.NotEqual(t, session.Value, storedSession.TokenHash)
	require.WithinDuration(t, time.Now().Add(30*24*time.Hour), storedSession.ExpiresAt, time.Minute)
}

func TestGitHubCallbackSecondLoginReusesUser(t *testing.T) {
	ctx := setupDB(t)
	stubApp(t, &github.App{ClientID: testClientID, ClientSecret: testClientSecret}, nil)
	stub := stubGitHubOAuth(t)

	cfg := fxtest.Configure()
	config.Set(cfg, github.URLConfig, stub.URL)
	config.Set(cfg, github.APIURLConfig, stub.URL)
	router := authRouter(t, cfg)

	first := loginCallback(t, router, ctx)
	second := loginCallback(t, router, ctx)
	require.NotEqual(t, first.Value, second.Value)

	var counts struct {
		Users      int `db:"users"`
		Identities int `db:"identities"`
		Sessions   int `db:"sessions"`
	}
	require.NoError(t, data.Get(ctx, &counts, `
		SELECT
			(SELECT count(*) FROM users) AS users,
			(SELECT count(*) FROM identities) AS identities,
			(SELECT count(*) FROM sessions) AS sessions`))
	require.Equal(t, 1, counts.Users)
	require.Equal(t, 1, counts.Identities)
	require.Equal(t, 2, counts.Sessions)
}

// TestUpsertGitHubUserFirstLoginRace simulates two concurrent first logins: a manual
// transaction plays the winner — its user+identity stay uncommitted while the loser's
// Execute starts, so the loser's SELECT misses the identity, takes the insert path,
// and hits the unique violation once the winner commits. Covers Execute's retry
// resolving to the winner's user. (If the loser's SELECT ever runs after the commit it
// degrades to the plain found path — still green, just not exercising the retry.)
func TestUpsertGitHubUserFirstLoginRace(t *testing.T) {
	ctx := setupDB(t)

	winner, err := data.FromContext(ctx).Beginx()
	require.NoError(t, err)
	var winnerID int64
	require.NoError(t, winner.Get(&winnerID,
		`INSERT INTO users (name) VALUES ('octocat') RETURNING id`))
	_, err = winner.Exec(`
		INSERT INTO identities (user_id, provider, provider_id, kind)
		VALUES ($1, 'github', '12345', 'login')`, winnerID)
	require.NoError(t, err)

	loser := &UpsertGitHubUser{
		Account: githubAccount{ID: 12345, Login: "octocat"},
		Token:   testAccessToken,
	}
	user := &User{}
	done := make(chan error, 1)
	go func() { done <- loser.Execute(ctx, user) }()

	time.Sleep(50 * time.Millisecond) // let Execute block on the identity insert
	require.NoError(t, winner.Commit())

	require.NoError(t, <-done)
	require.Equal(t, winnerID, user.ID)

	var counts struct {
		Users      int `db:"users"`
		Identities int `db:"identities"`
	}
	require.NoError(t, data.Get(ctx, &counts, `
		SELECT
			(SELECT count(*) FROM users) AS users,
			(SELECT count(*) FROM identities) AS identities`))
	require.Equal(t, 1, counts.Users)
	require.Equal(t, 1, counts.Identities)
}

func TestDeleteSessionInvalidatesSession(t *testing.T) {
	ctx := setupDB(t)
	_, token := startTestSession(t, ctx, time.Now().Add(time.Hour))
	router := authRouter(t, fxtest.Configure())

	req := httptest.NewRequest("DELETE", "/api/session", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req.WithContext(ctx))

	require.Equal(t, http.StatusOK, resp.Code)
	cleared := responseCookie(t, resp, sessionCookie)
	require.Empty(t, cleared.Value)
	require.Negative(t, cleared.MaxAge)

	var count int
	require.NoError(t, data.Get(ctx, &count, `SELECT count(*) FROM sessions`))
	require.Zero(t, count)

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, usersMeRequest(ctx, token))
	require.Equal(t, http.StatusUnauthorized, resp.Code)
}
