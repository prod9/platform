// Package auth owns platform identity: users and their linked provider identities,
// the GitHub App user-OAuth login flow, platform sessions, and the session gate
// other fragments put in front of their endpoints.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"fx.prodigy9.co/app"
	"fx.prodigy9.co/config"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/fxlog"
	"fx.prodigy9.co/httpserver/controllers"
	"fx.prodigy9.co/httpserver/httperrors"
	"fx.prodigy9.co/httpserver/render"
	"fx.prodigy9.co/secret"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"platform.prodigy9.co/srv/github"
)

var (
	App = app.Build().
		Name("auth").
		EmbedMigrations(Migrations).
		Controllers(SessionCtr{})

	ErrNoSession            = errors.New("auth: no session")
	errBadOAuthState        = errors.New("auth: oauth state mismatch")
	errInstallationNotBound = errors.New("auth: installation not bound")
	errNoOAuthToken         = errors.New("auth: oauth code exchange returned no access token")
	logOAuthFailure         = fxlog.Error
	loadBoundInstallationID = loadBoundInstallation
)

const (
	oauthStateCookie = "oauth_state"
	oauthStateTTL    = 10 * time.Minute

	sessionCookie = "platform_session"
	sessionTTL    = 2 * time.Hour
)

type Permission string

const (
	PermissionRead  Permission = "read"
	PermissionWrite Permission = "write"
)

// Repository is one repository authorized for the lifetime of a login session.
type Repository struct {
	GitHubID   int64      `db:"github_repository_id"`
	Owner      string     `db:"owner"`
	Name       string     `db:"name"`
	Permission Permission `db:"permission"`
}

// User is an internal platform user, the anchor of the identity ADR's model; external
// accounts link to it via identities rows.
type User struct {
	ID        int64     `db:"id"`
	Name      string    `db:"name"`
	CreatedAt time.Time `db:"created_at"`
}

// SystemUserID resolves the seeded system principal — the user a build triggered by the
// App itself is attributed to. It is looked up through its identity rather than by a
// literal id: the identity is what marks the row as the system's, and it is unique by the
// identities table's own constraint.
func SystemUserID(ctx context.Context) (int64, error) {
	var id int64
	err := data.Get(ctx, &id, `
		SELECT user_id FROM identities
		WHERE provider = 'system' AND provider_id = 'platform'`)
	return id, err
}

// Session is a live platform session's identity and lifetime — what the webui's
// validity probe needs, distinct from the user's profile.
type Session struct {
	ID        int64     `db:"session_id" json:"-"`
	UserID    int64     `db:"user_id"`
	ExpiresAt time.Time `db:"expires_at"`
}

// SessionCtr serves the GitHub App user-OAuth login flow (spec §Two token types: the
// user-to-server side), session validity (GET /api/session) and revocation
// (DELETE /api/session), and the session user's profile (GET /api/users/me).
// Platform issues its own session token per the identity ADR — the GitHub token is
// stored, never handed to the client.
type SessionCtr struct{}

var _ controllers.Interface = SessionCtr{}

func (SessionCtr) Mount(cfg *config.Source, router chi.Router) error {
	router.Get("/auth/github", githubLogin)
	router.Get("/auth/github/callback", githubLoginCallback)
	router.Get("/api/session", getSession)
	router.Delete("/api/session", deleteSession)
	router.Get("/api/users/me", usersMe)
	return nil
}

// CurrentUser resolves the platform session cookie to its unexpired session's user;
// anything short of that is ErrNoSession.
func CurrentUser(req *http.Request) (*User, error) {
	_, user, err := currentSession(req)
	return user, err
}

// CurrentSession resolves the platform session cookie to its unexpired session's
// user id and expiry; anything short of that is ErrNoSession.
func CurrentSession(req *http.Request) (*Session, error) {
	session, _, err := currentSession(req)
	return session, err
}

// currentSession is the one live-session lookup both public forms project from: the
// join already answers each, so neither needs its own query.
func currentSession(req *http.Request) (*Session, *User, error) {
	cookie, err := req.Cookie(sessionCookie)
	if err != nil || cookie.Value == "" {
		return nil, nil, ErrNoSession
	}

	row := struct {
		Session
		User
	}{}
	err = data.Get(req.Context(), &row, `
		SELECT sessions.id AS session_id, sessions.user_id, sessions.expires_at,
			users.id, users.name, users.created_at
		FROM sessions
		JOIN users ON users.id = sessions.user_id
		WHERE sessions.token_hash = $1 AND sessions.expires_at > now()`,
		hashSessionToken(cookie.Value))
	if data.IsNoRows(err) {
		return nil, nil, ErrNoSession
	} else if err != nil {
		return nil, nil, err
	}
	return &row.Session, &row.User, nil
}

// ReadableRepos returns the current session's complete local authorization snapshot.
func ReadableRepos(req *http.Request) ([]Repository, error) {
	session, err := CurrentSession(req)
	if err != nil {
		return nil, err
	}
	repositories := []Repository{}
	err = data.Select(req.Context(), &repositories, `
		SELECT github_repository_id, owner, name, permission
		FROM session_repositories WHERE session_id = $1
		ORDER BY owner, name`, session.ID)
	return repositories, err
}

func RequireRepoRead(resp http.ResponseWriter, req *http.Request, owner, name string) bool {
	return requireRepo(resp, req, owner, name, PermissionRead)
}

func RequireRepoWrite(resp http.ResponseWriter, req *http.Request, owner, name string) bool {
	return requireRepo(resp, req, owner, name, PermissionWrite)
}

func requireRepo(resp http.ResponseWriter, req *http.Request, owner, name string, required Permission) bool {
	session, err := CurrentSession(req)
	if errors.Is(err, ErrNoSession) {
		render.Error(resp, req, http.StatusUnauthorized, httperrors.ErrUnauthorized)
		return false
	} else if err != nil {
		render.Error(resp, req, http.StatusInternalServerError, err)
		return false
	}

	var permission Permission
	err = data.Get(req.Context(), &permission, `
		SELECT permission FROM session_repositories
		WHERE session_id = $1 AND lower(owner) = lower($2) AND lower(name) = lower($3)`,
		session.ID, owner, name)
	if data.IsNoRows(err) || (err == nil && required == PermissionWrite && permission != PermissionWrite) {
		render.Error(resp, req, http.StatusNotFound, httperrors.ErrNotFound)
		return false
	} else if err != nil {
		render.Error(resp, req, http.StatusInternalServerError, err)
		return false
	}
	return true
}

// RequireUser gates a handler on a live session: it resolves the current user or
// writes the failure response (401 no session, 500 otherwise) itself — ok=false
// means the response is already sent and the handler must return.
func RequireUser(resp http.ResponseWriter, req *http.Request) (*User, bool) {
	user, err := CurrentUser(req)
	if errors.Is(err, ErrNoSession) {
		render.Error(resp, req, 401, httperrors.ErrUnauthorized)
		return nil, false
	} else if err != nil {
		render.Error(resp, req, 500, err)
		return nil, false
	}
	return user, true
}

func usersMe(resp http.ResponseWriter, req *http.Request) {
	user, ok := RequireUser(resp, req)
	if !ok {
		return
	}

	render.JSON(resp, req, struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}{user.ID, user.Name})
}

func getSession(resp http.ResponseWriter, req *http.Request) {
	session, err := CurrentSession(req)
	if errors.Is(err, ErrNoSession) {
		render.Error(resp, req, 401, httperrors.ErrUnauthorized)
		return
	} else if err != nil {
		render.Error(resp, req, 500, err)
		return
	}

	render.JSON(resp, req, struct {
		UserID    int64     `json:"user_id"`
		ExpiresAt time.Time `json:"expires_at"`
	}{session.UserID, session.ExpiresAt})
}

func githubLogin(resp http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	serverURL, err := github.LoadPublicURL(ctx)
	if errors.Is(err, github.ErrNoPublicURL) {
		render.Error(resp, req, 503, err)
		return
	} else if err != nil {
		render.Error(resp, req, 500, err)
		return
	}
	serverURL = strings.TrimSuffix(serverURL, "/")

	app, err := github.LoadApp(ctx)
	if errors.Is(err, github.ErrNoApp) {
		render.Error(resp, req, 503, err)
		return
	} else if err != nil {
		render.Error(resp, req, 500, err)
		return
	}

	requestedInstallationID, _ := strconv.ParseInt(req.URL.Query().Get("installation_id"), 10, 64)
	installationID, err := resolveInstallationID(ctx, requestedInstallationID)
	if err != nil {
		render.Error(resp, req, http.StatusInternalServerError, err)
		return
	}
	bound := oauthState{Nonce: randomToken(), Return: safeReturn(req.URL.Query().Get("return"))}
	if installationID > 0 {
		bound.InstallationID = installationID
	}
	state := encodeOAuthState(bound)
	http.SetCookie(resp, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    state,
		Path:     "/auth",
		MaxAge:   int(oauthStateTTL.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	query := url.Values{
		"client_id":    {app.ClientID},
		"redirect_uri": {serverURL + "/auth/github/callback"},
		"state":        {bound.Nonce},
	}
	githubURL := config.Get(config.FromContext(ctx), github.URLConfig)
	render.Redirect(resp, req, githubURL+"/login/oauth/authorize?"+query.Encode())
}

func githubLoginCallback(resp http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	cfg := config.FromContext(ctx)

	state := req.URL.Query().Get("state")
	stateCookie, err := req.Cookie(oauthStateCookie)
	bound, decodeErr := decodeOAuthState(stateCookieValue(stateCookie, err))
	if state == "" || decodeErr != nil || bound.Nonce != state {
		failOAuth(resp, req, bound, errBadOAuthState)
		return
	}
	if req.URL.Query().Get("error") != "" || req.URL.Query().Get("code") == "" {
		failOAuth(resp, req, bound, fmt.Errorf("auth: oauth callback denied or missing code: %s",
			req.URL.Query().Get("error")))
		return
	}

	app, err := github.LoadApp(ctx)
	if err != nil {
		failOAuth(resp, req, bound, err)
		return
	}

	githubURL := config.Get(cfg, github.URLConfig)
	token, err := exchangeOAuthCode(ctx, http.DefaultClient, githubURL,
		app.ClientID, app.ClientSecret, req.URL.Query().Get("code"))
	if err != nil {
		failOAuth(resp, req, bound, err)
		return
	}

	apiURL := config.Get(cfg, github.APIURLConfig)
	account, err := fetchGitHubUser(ctx, http.DefaultClient, apiURL, token)
	if err != nil {
		failOAuth(resp, req, bound, err)
		return
	}

	installationID, err := resolveInstallationID(ctx, bound.InstallationID)
	if err != nil {
		failOAuth(resp, req, bound, err)
		return
	}
	bound.InstallationID = installationID
	client, err := github.NewClient(ctx)
	if err != nil {
		failOAuth(resp, req, bound, err)
		return
	}
	repositories, err := client.UserInstallationRepos(ctx, token, installationID)
	if err != nil {
		failOAuth(resp, req, bound, err)
		return
	}

	sessionToken := randomToken()
	login := &CreateLogin{
		Account:      *account,
		GitHubToken:  token,
		Repositories: repositories,
		SessionToken: sessionToken,
		ExpiresAt:    time.Now().Add(sessionTTL),
	}
	if err := login.Execute(ctx, &User{}); err != nil {
		failOAuth(resp, req, bound, err)
		return
	}

	http.SetCookie(resp, &http.Cookie{
		Name:     sessionCookie,
		Value:    sessionToken,
		Path:     "/",
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
	// the state cookie is single-use; a successful callback consumes it.
	http.SetCookie(resp, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    "",
		Path:     "/auth",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
	render.Redirect(resp, req, bound.Return)
}

type oauthState struct {
	Nonce          string `json:"nonce"`
	Return         string `json:"return"`
	InstallationID int64  `json:"installation_id"`
}

func encodeOAuthState(state oauthState) string {
	raw, err := json.Marshal(state)
	if err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeOAuthState(value string) (oauthState, error) {
	state := oauthState{}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return state, errBadOAuthState
	}
	if err := json.Unmarshal(raw, &state); err != nil {
		return state, errBadOAuthState
	}
	return state, nil
}

func stateCookieValue(cookie *http.Cookie, err error) string {
	if err != nil || cookie == nil {
		return ""
	}
	return cookie.Value
}

func safeReturn(value string) string {
	parsed, err := url.Parse(value)
	if err != nil || value == "" || strings.HasPrefix(value, "//") ||
		!strings.HasPrefix(parsed.Path, "/") || strings.HasPrefix(parsed.Path, "//") ||
		parsed.IsAbs() || parsed.Host != "" {
		return "/"
	}
	return parsed.RequestURI()
}

func redirectSignIn(resp http.ResponseWriter, req *http.Request, returnTo string, installationID int64) {
	query := url.Values{"return": {safeReturn(returnTo)}}
	if installationID > 0 {
		query.Set("installation_id", strconv.FormatInt(installationID, 10))
	}
	render.Redirect(resp, req, "/session/?"+query.Encode())
}

func failOAuth(resp http.ResponseWriter, req *http.Request, bound oauthState, err error) {
	logOAuthFailure(err)
	redirectSignIn(resp, req, safeReturn(bound.Return), bound.InstallationID)
}

func loadBoundInstallation(ctx context.Context) (int64, error) {
	if _, ok := data.LookupFromContext(ctx); !ok {
		return 0, errInstallationNotBound
	}
	value, err := github.LoadSetting(ctx, "install.installation_id", errInstallationNotBound)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(value, 10, 64)
}

func resolveInstallationID(ctx context.Context, requested int64) (int64, error) {
	bound, err := loadBoundInstallationID(ctx)
	if errors.Is(err, errInstallationNotBound) {
		return requested, nil
	}
	return bound, err
}

func deleteSession(resp http.ResponseWriter, req *http.Request) {
	cookie, err := req.Cookie(sessionCookie)
	if err == nil && cookie.Value != "" {
		del := &DeleteSession{Token: cookie.Value}
		if err := del.Execute(req.Context(), nil); err != nil {
			render.Error(resp, req, 500, err)
			return
		}
	}

	http.SetCookie(resp, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
	render.JSON(resp, req, struct {
		Status string `json:"status"`
	}{"logged_out"})
}

// oauthTokenResponse is GitHub's access-token exchange response (JSON form, via the
// Accept header). OAuth errors come back as 200s with an error field.
type oauthTokenResponse struct {
	AccessToken      string `json:"access_token"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func exchangeOAuthCode(ctx context.Context, client *http.Client, githubURL, clientID, clientSecret, code string) (string, error) {
	form := url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"code":          {code},
	}
	exchangeURL := strings.TrimSuffix(githubURL, "/") + "/login/oauth/access_token"
	req, err := http.NewRequestWithContext(ctx, "POST", exchangeURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", github.RespError("oauth code exchange", resp)
	}

	token := oauthTokenResponse{}
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return "", err
	}
	if token.Error != "" {
		return "", fmt.Errorf("auth: oauth code exchange failed: %s: %s",
			token.Error, token.ErrorDescription)
	}
	if token.AccessToken == "" {
		return "", errNoOAuthToken
	}
	return token.AccessToken, nil
}

// GitHubAccount is the subset of GET /user the login flow needs. GitHub reports null
// for a hidden email; null decodes to "".
type GitHubAccount struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Email string `json:"email"`
}

func fetchGitHubUser(ctx context.Context, client *http.Client, apiURL, token string) (*GitHubAccount, error) {
	userURL := strings.TrimSuffix(apiURL, "/") + "/user"
	req, err := http.NewRequestWithContext(ctx, "GET", userURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, github.RespError("fetching github user", resp)
	}

	account := &GitHubAccount{}
	if err := json.NewDecoder(resp.Body).Decode(account); err != nil {
		return nil, err
	}
	return account, nil
}

// CreateLogin atomically refreshes the GitHub identity and records the bounded
// session authorization assembled from GitHub before the transaction begins.
type CreateLogin struct {
	Account      GitHubAccount
	GitHubToken  string
	Repositories []github.Repo
	SessionToken string
	ExpiresAt    time.Time
}

func (c *CreateLogin) Execute(ctx context.Context, out any) error {
	err := c.executeOnce(ctx, out)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return c.executeOnce(ctx, out)
	}
	return err
}

func (c *CreateLogin) executeOnce(ctx context.Context, out any) error {
	ciphertext, err := secret.Hide(config.FromContext(ctx), c.GitHubToken)
	if err != nil {
		return err
	}
	metadata, err := json.Marshal(map[string]string{
		"login": c.Account.Login, "token": ciphertext,
	})
	if err != nil {
		return err
	}
	providerID := strconv.FormatInt(c.Account.ID, 10)

	return data.Run(ctx, func(scope data.Scope) error {
		var userID int64
		err := scope.Get(&userID, `SELECT user_id FROM identities
			WHERE provider = 'github' AND provider_id = $1`, providerID)
		if data.IsNoRows(err) {
			if err = scope.Get(&userID,
				`INSERT INTO users (name) VALUES ($1) RETURNING id`, c.Account.Login); err != nil {
				return err
			}
			if err = scope.Exec(`INSERT INTO identities
				(user_id, provider, provider_id, kind, email, email_verified, metadata)
				VALUES ($1, 'github', $2, 'login', $3, false, $4)`,
				userID, providerID, c.Account.Email, string(metadata)); err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else {
			if err = scope.Exec(`UPDATE users SET name = $2 WHERE id = $1`,
				userID, c.Account.Login); err != nil {
				return err
			}
			if err = scope.Exec(`UPDATE identities SET email = $2, metadata = $3
				WHERE user_id = $1 AND provider = 'github' AND kind = 'login'`,
				userID, c.Account.Email, string(metadata)); err != nil {
				return err
			}
		}

		var sessionID int64
		if err = scope.Get(&sessionID, `INSERT INTO sessions
			(user_id, token_hash, expires_at) VALUES ($1, $2, $3) RETURNING id`,
			userID, hashSessionToken(c.SessionToken), c.ExpiresAt); err != nil {
			return err
		}
		for _, repository := range c.Repositories {
			permission := PermissionRead
			if repository.Permission == github.RepoWrite {
				permission = PermissionWrite
			}
			if err = scope.Exec(`INSERT INTO session_repositories
				(session_id, github_repository_id, owner, name, permission)
				VALUES ($1, $2, $3, $4, $5)`, sessionID, repository.ID,
				repository.Owner, repository.Name, permission); err != nil {
				return err
			}
		}
		return scope.Get(out, `SELECT * FROM users WHERE id = $1`, userID)
	})
}

// CreateSession records a platform session for a raw token. Hashing is the store's
// own invariant: the client keeps the raw token in the session cookie; only its
// SHA-256 lands in the database.
type CreateSession struct {
	UserID    int64
	Token     string
	ExpiresAt time.Time
}

func (c *CreateSession) Execute(ctx context.Context, out any) error {
	return data.Exec(ctx, `
		INSERT INTO sessions (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)`,
		c.UserID, hashSessionToken(c.Token), c.ExpiresAt)
}

// DeleteSession revokes one session by its raw token; an already-gone session
// deletes as a no-op.
type DeleteSession struct {
	Token string
}

func (d *DeleteSession) Execute(ctx context.Context, out any) error {
	return data.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, hashSessionToken(d.Token))
}

// randomToken returns 32 crypto/rand bytes hex-encoded — the shape of both OAuth
// states and session tokens.
func randomToken() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		panic(err) // crypto/rand.Read never fails (Go 1.24+)
	}
	return hex.EncodeToString(buf)
}

// hashSessionToken derives a session token's at-rest form; the raw token never
// touches the database.
func hashSessionToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
