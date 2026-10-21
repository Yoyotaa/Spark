//go:build !darwin && !windows

package tray

import "errors"

func start(string, []Item) error { return errors.New("tray: unsupported platform") }
