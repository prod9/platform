package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// request describes one wire call — everything an endpoint differs by. The
// transport owns auth-header selection, JSON decoding, error shaping, and
// pagination; endpoint methods declare only these fields
// (docs/spec/platform-server.md, the transport layer).
type request struct {
	method string
	path   string
	auth   bearer
	accept string
	op     string
	status map[int]error
}

// bearer supplies the Authorization credential at send time.
type bearer func() (string, error)

func (c *Client) asApp() bearer {
	return func() (string, error) { return c.app.jwt(time.Now()) }
}

func asToken(token string) bearer {
	return func() (string, error) { return token, nil }
}

// fetchJSON performs the request and decodes the 2xx body into T.
func fetchJSON[T any](ctx context.Context, c *Client, req request) (T, error) {
	var decoded T

	body, err := fetchRaw(ctx, c, req)
	if err != nil {
		return decoded, err
	}

	if err := json.Unmarshal(body, &decoded); err != nil {
		return decoded, fmt.Errorf("github: decoding %s: %w", req.op, err)
	}
	return decoded, nil
}

// fetchRaw performs the request and returns the 2xx body as-is. A status listed in
// req.status answers as its sentinel error; any other non-2xx is a RespError.
func fetchRaw(ctx context.Context, c *Client, req request) ([]byte, error) {
	resp, err := send(ctx, c, req, c.apiURL+req.path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := checkStatus(req, resp); err != nil {
		return nil, err
	}
	return io.ReadAll(resp.Body)
}

// fetchPaged walks a paginated endpoint: each page decodes into P and drains, and
// the walk follows the Link header's own URLs until a response carries no
// rel="next" — the protocol's end-of-list signal, never a page count or cap
// (docs/vendor/github-app-api.md §Pagination).
func fetchPaged[P any](ctx context.Context, c *Client, req request, drain func(P)) error {
	for url := c.apiURL + req.path; url != ""; {
		resp, err := send(ctx, c, req, url)
		if err != nil {
			return err
		}

		page, err := drainPage[P](req, resp)
		if err != nil {
			return err
		}
		drain(page)

		url = nextLink(resp)
	}
	return nil
}

// drainPage consumes one paginated response; split out so the body closes per page.
func drainPage[P any](req request, resp *http.Response) (P, error) {
	var page P
	defer resp.Body.Close()

	if err := checkStatus(req, resp); err != nil {
		return page, err
	}

	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return page, fmt.Errorf("github: decoding %s: %w", req.op, err)
	}
	return page, nil
}

func send(ctx context.Context, c *Client, req request, url string) (*http.Response, error) {
	credential, err := req.auth()
	if err != nil {
		return nil, err
	}

	wire, err := http.NewRequestWithContext(ctx, req.method, url, nil)
	if err != nil {
		return nil, err
	}
	wire.Header.Set("Authorization", "Bearer "+credential)
	if req.accept != "" {
		wire.Header.Set("Accept", req.accept)
	}

	return httpClient.Do(wire)
}

func checkStatus(req request, resp *http.Response) error {
	if mapped, listed := req.status[resp.StatusCode]; listed {
		return mapped
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return RespError(req.op, resp)
	}
	return nil
}

// nextLink extracts the rel="next" URL from the response's Link header; empty when
// the header (or the rel) is absent — the last page.
func nextLink(resp *http.Response) string {
	for entry := range strings.SplitSeq(resp.Header.Get("Link"), ",") {
		url, rel, found := strings.Cut(entry, ";")
		if !found {
			continue
		}

		if strings.TrimSpace(rel) == `rel="next"` {
			return strings.Trim(strings.TrimSpace(url), "<>")
		}
	}
	return ""
}
