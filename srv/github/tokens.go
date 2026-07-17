package github

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ErrAppNotInstalled reports a repo the stored GitHub App has no installation on —
// either token type only reaches installed repos (spec §Constraints to design around).
var ErrAppNotInstalled = errors.New("github: app not installed")

// appJWT mints the App-authentication JWT GitHub expects on App endpoints. Wire
// shape: base64url (no padding) of header {"alg":"RS256","typ":"JWT"} and claims
// {iat: now-60s, exp: now+9m, iss: app id as string} joined with '.', then the
// RSASSA-PKCS1-v1_5/SHA-256 signature over that joined string appended the same way.
// The App key is PKCS#1 PEM as GitHub issues it. One sign operation — deliberately
// hand-rolled to keep a JWT dependency out.
func appJWT(app *App, now time.Time) (string, error) {
	block, _ := pem.Decode([]byte(app.PrivateKey))
	if block == nil {
		return "", errors.New("github: app private key is not PEM")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("github: parsing app private key: %w", err)
	}

	claims, err := json.Marshal(struct {
		Iat int64  `json:"iat"`
		Exp int64  `json:"exp"`
		Iss string `json:"iss"`
	}{
		Iat: now.Add(-time.Minute).Unix(),
		Exp: now.Add(9 * time.Minute).Unix(),
		Iss: strconv.FormatInt(app.AppID, 10),
	})
	if err != nil {
		return "", err
	}

	encode := base64.RawURLEncoding.EncodeToString
	signing := encode([]byte(`{"alg":"RS256","typ":"JWT"}`)) + "." + encode(claims)
	digest := sha256.Sum256([]byte(signing))
	signature, err := rsa.SignPKCS1v15(nil, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return signing + "." + encode(signature), nil
}

// MintInstallationToken exchanges the App JWT for a short-lived installation token
// scoped to owner/repo's installation (spec §Two token types): the repo installation
// lookup resolves the installation id, then the access-token create mints the token.
func MintInstallationToken(ctx context.Context, client *http.Client, apiURL string, app *App, owner, repo string) (string, error) {
	jwt, err := appJWT(app, time.Now())
	if err != nil {
		return "", err
	}
	base := strings.TrimSuffix(apiURL, "/")

	installation := struct {
		ID int64 `json:"id"`
	}{}
	status, err := appCall(ctx, client, "GET",
		base+"/repos/"+owner+"/"+repo+"/installation", jwt, http.StatusOK, &installation)
	if status == http.StatusNotFound {
		return "", fmt.Errorf("%w on %s/%s", ErrAppNotInstalled, owner, repo)
	} else if err != nil {
		return "", err
	}

	token := struct {
		Token string `json:"token"`
	}{}
	_, err = appCall(ctx, client, "POST",
		base+"/app/installations/"+strconv.FormatInt(installation.ID, 10)+"/access_tokens",
		jwt, http.StatusCreated, &token)
	if err != nil {
		return "", err
	}
	return token.Token, nil
}

// appCall runs one JWT-authenticated, bodyless GitHub API call, decoding the JSON
// response into out on the wanted status and returning the actual status either way
// so callers can special-case (the 404 → not-installed mapping).
func appCall(ctx context.Context, client *http.Client, method, url, jwt string, want int, out any) (int, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+jwt)

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != want {
		return resp.StatusCode, RespError(method+" "+url, resp)
	}

	return resp.StatusCode, json.NewDecoder(resp.Body).Decode(out)
}

// RespError summarizes a failed GitHub API response: op, status line, and up to 1KB
// of body. The body read is best-effort — GitHub's status already carries the
// verdict, so a read failure only shortens the message, never masks it.
func RespError(op string, resp *http.Response) error {
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<10))
	if err != nil {
		body = []byte("(unreadable body: " + err.Error() + ")")
	}
	return fmt.Errorf("github: %s failed: %d %s: %s",
		op, resp.StatusCode, resp.Status, body)
}
