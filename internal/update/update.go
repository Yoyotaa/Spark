// Package update checks GitHub for a newer release of Spark.
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

const latestURL = "https://api.github.com/repos/Yoyotaa/Spark/releases/latest"

// Release is a published version.
type Release struct {
	Version string `json:"version"` // without the leading "v"
	URL     string `json:"url"`     // release page
}

// Latest returns the latest published release. baseURL overrides the API
// endpoint (tests).
func Latest(ctx context.Context, client *http.Client, baseURL string) (Release, error) {
	if baseURL == "" {
		baseURL = latestURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL, nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("github: %s", resp.Status)
	}
	var r struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return Release{}, err
	}
	return Release{Version: strings.TrimPrefix(r.TagName, "v"), URL: r.HTMLURL}, nil
}

// Newer reports whether version a is strictly newer than b ("1.10.0" >
// "1.9.2"). Pre-release suffixes are ignored; unparsable parts count as 0.
func Newer(a, b string) bool {
	pa, pb := parts(a), parts(b)
	for i := range 3 {
		if pa[i] != pb[i] {
			return pa[i] > pb[i]
		}
	}
	return false
}

func parts(v string) [3]int {
	v = strings.TrimPrefix(v, "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	var out [3]int
	for i, p := range strings.SplitN(v, ".", 3) {
		out[i], _ = strconv.Atoi(p)
	}
	return out
}
