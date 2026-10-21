//go:build !darwin || bindings

package selfresp

// Relaunch is only needed on macOS (and never while Wails generates the
// bindings).
func Relaunch() (code int, relaunched bool) { return 0, false }
