//go:build !darwin && !windows

package startup

import "errors"

func enabled() bool { return false }

func set(bool) error { return errors.New("non pris en charge sur cette plateforme") }
