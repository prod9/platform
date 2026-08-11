package github

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNextLink(t *testing.T) {
	cases := []struct {
		name   string
		header string
		next   string
	}{
		{"absent header is the end", "", ""},
		{"next among rels",
			`<https://x.test/a?page=2>; rel="next", <https://x.test/a?page=9>; rel="last"`,
			"https://x.test/a?page=2"},
		{"no next on the last page",
			`<https://x.test/a?page=8>; rel="prev", <https://x.test/a?page=1>; rel="first"`,
			""},
		{"cursor-style url is passed through verbatim",
			`<https://x.test/a?after=opaque%3Dcursor>; rel="next"`,
			"https://x.test/a?after=opaque%3Dcursor"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := &http.Response{Header: http.Header{}}
			if tc.header != "" {
				resp.Header.Set("Link", tc.header)
			}
			require.Equal(t, tc.next, nextLink(resp))
		})
	}
}

// The walk follows the Link header's own URLs — a next URL whose query params share
// nothing with the first request's still reaches the server untouched, and the walk
// stops at the response that carries no rel="next".
func TestFetchPagedFollowsLinkURLsVerbatim(t *testing.T) {
	client, _ := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer ghs_tok", r.Header.Get("Authorization"))

		switch r.URL.RequestURI() {
		case "/items?per_page=2":
			w.Header().Set("Link", fmt.Sprintf(`<http://%s/items?cursor=abc>; rel="next"`, r.Host))
			fmt.Fprint(w, `["a","b"]`)
		case "/items?cursor=abc":
			w.Header().Set("Link", fmt.Sprintf(`<http://%s/items?cursor=abc>; rel="prev"`, r.Host))
			fmt.Fprint(w, `["c"]`)
		default:
			t.Errorf("unexpected request %s", r.URL.RequestURI())
		}
	}))

	var got []string
	err := fetchPaged(context.Background(), client,
		request{method: "GET", path: "/items?per_page=2", auth: asToken("ghs_tok"), op: "item list"},
		func(page []string) { got = append(got, page...) })
	require.NoError(t, err)
	require.Equal(t, []string{"a", "b", "c"}, got)
}

func TestFetchPagedSurfacesMidWalkError(t *testing.T) {
	client, _ := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RequestURI() == "/items" {
			w.Header().Set("Link", fmt.Sprintf(`<http://%s/items?page=2>; rel="next"`, r.Host))
			fmt.Fprint(w, `["a"]`)
			return
		}
		w.WriteHeader(500)
		fmt.Fprint(w, `{"message":"boom"}`)
	}))

	err := fetchPaged(context.Background(), client,
		request{method: "GET", path: "/items", auth: asToken("ghs_tok"), op: "item list"},
		func([]string) {})
	require.ErrorContains(t, err, "boom")
}

// A status listed in the request's map answers as its sentinel — the transport owns
// the mapping so endpoints stay branch-free.
func TestFetchJSONMapsListedStatuses(t *testing.T) {
	sentinel := fmt.Errorf("mapped")
	client, _ := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))

	_, err := fetchJSON[struct{}](context.Background(), client,
		request{method: "GET", path: "/thing", auth: asToken("t"), op: "thing lookup",
			status: map[int]error{404: sentinel}})
	require.ErrorIs(t, err, sentinel)
}
