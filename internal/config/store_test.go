package config

import (
	"path/filepath"
	"testing"
)

func TestStoreRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())    // macOS / Linux
	t.Setenv("XDG_CONFIG_HOME", "")  // Linux
	t.Setenv("AppData", t.TempDir()) // Windows

	st, err := OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if st.Get().Orb != nil {
		t.Fatal("fresh store should have no orb position")
	}
	if err := st.Update(func(s *Settings) { s.Orb = &Point{X: 12, Y: 34} }); err != nil {
		t.Fatal(err)
	}

	again, err := OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if p := again.Get().Orb; p == nil || *p != (Point{12, 34}) {
		t.Fatalf("got %+v from %s", p, filepath.Dir(again.path))
	}
}
