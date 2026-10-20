package config

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

// Point is a position in CSS pixels.
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Settings are the user preferences persisted between launches.
type Settings struct {
	Orb *Point `json:"orb,omitempty"` // A.R.I.A button position
}

// Store persists Settings as JSON in the OS config directory
// (~/Library/Application Support/Spark on macOS, %AppData%\Spark on Windows).
type Store struct {
	path string

	mu sync.Mutex
	s  Settings
}

func OpenStore() (*Store, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	st := &Store{path: filepath.Join(dir, "Spark", "settings.json")}
	data, err := os.ReadFile(st.path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return st, nil
	case err != nil:
		return nil, err
	}
	// A corrupted file must not prevent the app from starting.
	_ = json.Unmarshal(data, &st.s)
	return st, nil
}

func (st *Store) Get() Settings {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.s
}

// Update applies fn and writes the result atomically (temp file + rename),
// so a crash mid-write never leaves a truncated file.
func (st *Store) Update(fn func(*Settings)) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	fn(&st.s)

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
