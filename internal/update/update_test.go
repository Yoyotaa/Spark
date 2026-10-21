package update

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewer(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"0.2.0", "0.1.0", true}, {"1.10.0", "1.9.9", true}, {"v1.0.0", "1.0.0", false},
		{"1.0.0", "1.0.1", false}, {"2.0.0-beta", "1.9.0", true}, {"0.1.0", "dev", true},
	} {
		if got := Newer(c.a, c.b); got != c.want {
			t.Errorf("Newer(%q, %q) = %v", c.a, c.b, got)
		}
	}
}

func TestLatest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"tag_name":"v0.3.1","html_url":"https://github.com/Yoyotaa/Spark/releases/tag/v0.3.1"}`)
	}))
	defer srv.Close()
	r, err := Latest(context.Background(), srv.Client(), srv.URL)
	if err != nil || r.Version != "0.3.1" || r.URL == "" {
		t.Fatalf("got %+v, %v", r, err)
	}
}
