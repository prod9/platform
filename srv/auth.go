package srv

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/httpserver/controllers"
	"fx.prodigy9.co/httpserver/render"
	"fx.prodigy9.co/secret"
	"github.com/go-chi/chi/v5"
)

const (
	oauthStateCookie = "oauth_state"
	oauthStateTTL    = 10 * time.Minute

	sessionCookie = "platform_session"
	sessionTTL    = 30 * 24 * time.Hour
)

var (
	ErrNoSession     = errors.New("srv: no session")
	errBadOAuthState = errors.New("srv: oauth state mismatch")
	errNoOAuthToken  = errors.New("srv: oauth code exchange returned no access token")
)

// User is an internal platform user, the anchor of the identity ADR's model; external
// accounts link to it via identities rows.
type User struct {
	ID        int64     `db:"id"`
	Name      string    `db:"name"`
	CreatedAt time.Time `db:"created_at"`
}

// Auth serves the GitHub App user-OAuth login flow (spec §Two token types: the
// user-to-server side) and platform session logout. Platform issues its own session
// token per the identity ADR — the GitHub token is stored, never handed to the client.
type Auth struct{}

var _ controllers.Interface = Auth{}

func (Auth) Mount(cfg *config.Source, router chi.Router) error {
	router.Get("/api/auth/github", githubLogin)
	router.Get("/api/auth/github/callback", githubLoginCallback)
	router.Post("/api/auth/logout", logout)
	return nil
}

func githubLogin(resp http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	cfg := config.FromContext(ctx)

	serverURL, ok := config.GetOK(cfg, ServerURLConfig)
	if !ok {
		render.Error(resp, req, 500, errors.New("srv: SERVER_URL must be set for GitHub login"))
		return
	}
	serverURL = strings.TrimSuffix(serverURL, "/")

	app, err := loadGitHubApp(ctx)
	if errors.Is(err, ErrNoGitHubApp) {
		render.Error(resp, req, 503, err)
		return
	} else if err != nil {
		render.Error(resp, req, 500, err)
		return
	}

	state := randomToken()
	http.SetCookie(resp, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    state,
		Path:     "/api/auth",
		MaxAge:   int(oauthStateTTL.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	query := url.Values{
		"client_id":    {app.ClientID},
		"redirect_uri": {serverURL + "/api/auth/github/callback"},
		"state":        {state},
	}
	githubURL := config.Get(cfg, GitHubURLConfig)
	render.Redirect(resp, req, githubURL+"/login/oauth/authorize?"+query.Encode())
}

func githubLoginCallback(resp http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	cfg := config.FromContext(ctx)

	state := req.URL.Query().Get("state")
	stateCookie, err := req.Cookie(oauthStateCookie)
	if state == "" || err != nil || stateCookie.Value != state {
		render.Error(resp, req, 400, errBadOAuthState)
		return
	}

	app, err := loadGitHubApp(ctx)
	if errors.Is(err, ErrNoGitHubApp) {
		render.Error(resp, req, 503, err)
		return
	} else if err != nil {
		render.Error(resp, req, 500, err)
		return
	}

	githubURL := config.Get(cfg, GitHubURLConfig)
	token, err := exchangeOAuthCode(ctx, http.DefaultClient, githubURL,
		app.ClientID, app.ClientSecret, req.URL.Query().Get("code"))
	if err != nil {
		render.Error(resp, req, 502, err)
		return
	}

	apiURL := config.Get(cfg, GitHubAPIURLConfig)
	account, err := fetchGitHubUser(ctx, http.DefaultClient, apiURL, token)
	if err != nil {
		render.Error(resp, req, 502, err)
		return
	}

	upsert, user := &UpsertGitHubUser{Account: *account, Token: token}, &User{}
	if err := upsert.Execute(ctx, user); err != nil {
		render.Error(resp, req, 500, err)
		return
	}

	sessionToken := randomToken()
	create := &CreateSession{
		UserID:    user.ID,
		TokenHash: hashSessionToken(sessionToken),
		ExpiresAt: time.Now().Add(sessionTTL),
	}
	if err := create.Execute(ctx, nil); err != nil {
		render.Error(resp, req, 500, err)
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
	render.Redirect(resp, req, "/")
}

func logout(resp http.ResponseWriter, req *http.Request) {
	cookie, err := req.Cookie(sessionCookie)
	if err == nil && cookie.Value != "" {
		del := &DeleteSession{TokenHash: hashSessionToken(cookie.Value)}
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
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<10))
		return "", fmt.Errorf("srv: oauth code exchange failed: %d %s: %s",
			resp.StatusCode, resp.Status, body)
	}

	token := oauthTokenResponse{}
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return "", err
	}
	if token.Error != "" {
		return "", fmt.Errorf("srv: oauth code exchange failed: %s: %s",
			token.Error, token.ErrorDescription)
	}
	if token.AccessToken == "" {
		return "", errNoOAuthToken
	}
	return token.AccessToken, nil
}

// githubAccount is the subset of GET /user the login flow needs. GitHub reports null
// for a hidden email; null decodes to "".
type githubAccount struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Email string `json:"email"`
}

func fetchGitHubUser(ctx context.Context, client *http.Client, apiURL, token string) (*githubAccount, error) {
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
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<10))
		return nil, fmt.Errorf("srv: fetching github user failed: %d %s: %s",
			resp.StatusCode, resp.Status, body)
	}

	account := &githubAccount{}
	if err := json.NewDecoder(resp.Body).Decode(account); err != nil {
		return nil, err
	}
	return account, nil
}

// UpsertGitHubUser finds the platform user linked to a GitHub account by the
// immutable provider id (renames don't break links — the login lives in metadata,
// per the identity ADR) or creates user + identity on first login. The user token is
// stored encrypted in identity metadata. Token refresh and verified-email auto-link
// are later slices: an existing identity is matched, never updated, and no email
// lookup happens.
type UpsertGitHubUser struct {
	Account githubAccount
	Token   string
}

func (u *UpsertGitHubUser) Execute(ctx context.Context, out any) error {
	cfg := config.FromContext(ctx)
	token, err := secret.Hide(cfg, u.Token)
	if err != nil {
		return err
	}
	metadata, err := json.Marshal(map[string]string{"login": u.Account.Login, "token": token})
	if err != nil {
		return err
	}
	providerID := strconv.FormatInt(u.Account.ID, 10)

	return data.Run(ctx, func(scope data.Scope) error {
		var userID int64
		err := scope.Get(&userID, `
			SELECT user_id FROM identities
			WHERE provider = 'github' AND provider_id = $1`, providerID)
		if data.IsNoRows(err) {
			err = scope.Get(&userID,
				`INSERT INTO users (name) VALUES ($1) RETURNING id`, u.Account.Login)
			if err != nil {
				return err
			}
			err = scope.Exec(`
				INSERT INTO identities (user_id, provider, provider_id, kind, email, email_verified, metadata)
				VALUES ($1, 'github', $2, 'login', $3, false, $4)`,
				userID, providerID, u.Account.Email, string(metadata))
			if err != nil {
				return err
			}
		} else if err != nil {
			return err
		}

		return scope.Get(out, `SELECT * FROM users WHERE id = $1`, userID)
	})
}

// CreateSession records a platform session. The client keeps the raw token in the
// session cookie; only its SHA-256 lands in the database.
type CreateSession struct {
	UserID    int64
	TokenHash string
	ExpiresAt time.Time
}

func (c *CreateSession) Execute(ctx context.Context, out any) error {
	return data.Exec(ctx, `
		INSERT INTO sessions (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)`,
		c.UserID, c.TokenHash, c.ExpiresAt)
}

// DeleteSession revokes one session by its token hash; an already-gone session
// deletes as a no-op.
type DeleteSession struct {
	TokenHash string
}

func (d *DeleteSession) Execute(ctx context.Context, out any) error {
	return data.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, d.TokenHash)
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
