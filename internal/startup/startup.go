// Package startup registers Spark to launch when the user logs in.
package startup

// Enabled reports whether Spark launches at login.
func Enabled() bool { return enabled() }

// Set enables or disables the launch at login.
func Set(on bool) error { return set(on) }
