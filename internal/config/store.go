package config

import (
	"encoding/json"
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"sync"
)

// Point is a position in CSS pixels.
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Store persists Settings as JSON in the OS config directory
// (~/Library/Application Support/Spark on macOS, %AppData%\Spark on Windows).
// With an empty path (config dir unusable) it only keeps them in memory.
type Store struct {
	path string

	mu sync.Mutex
	s  Settings
}

// OpenStore loads the settings file. It always returns a usable store: on
// error the settings stay in memory and the error explains why.
func OpenStore() (*Store, error) {
	st := &Store{s: Defaults()}
	dir, err := os.UserConfigDir()
	if err != nil {
		return st, err
	}
	st.path = filepath.Join(dir, "Spark", "settings.json")
	data, err := os.ReadFile(st.path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return st, nil
	case err != nil:
		return st, err
	}
	// Decoding over the defaults keeps them for fields the file lacks
	// (older versions). A corrupted file must not prevent the app from
	// starting: the defaults are used.
	_ = json.Unmarshal(data, &st.s)
	st.s = st.s.Normalize()
	return st, nil
}

// Path is the settings file location ("" if in memory only).
func (st *Store) Path() string { return st.path }

func (st *Store) Get() Settings {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.s.clone()
}

func (s Settings) clone() Settings {
	s.AI.Models = maps.Clone(s.AI.Models)
	s.AI.BaseURLs = maps.Clone(s.AI.BaseURLs)
	if s.Orb != nil {
		p := *s.Orb
		s.Orb = &p
	}
	return s
}

// Update applies fn and writes the result atomically (temp file + rename),
// so a crash mid-write never leaves a truncated file.
func (st *Store) Update(fn func(*Settings)) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	next := st.s.clone()
	fn(&next)
	st.s = next.Normalize()
	if st.path == "" {
		return nil
	}

	data, err := json.MarshalIndent(st.s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(st.path), 0o755); err != nil {
		return err
	}
	tmp := st.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, st.path)
}
