//go:build !darwin && !windows

package overlay

import "errors"

// Linux is not a target yet: the app runs as a regular window.
type nopPlatform struct{}

func newPlatform() platform { return nopPlatform{} }

func (nopPlatform) attach() error                    { return errors.New("overlay: unsupported platform") }
func (nopPlatform) cursor() (float64, float64, bool) { return 0, 0, false }
func (nopPlatform) setIgnoreMouse(bool)              {}
func (nopPlatform) focus()                           {}
func (nopPlatform) restoreFocus()                    {}
func (nopPlatform) ensureFront() int                 { return 0 }
