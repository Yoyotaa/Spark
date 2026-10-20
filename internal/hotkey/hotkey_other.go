//go:build !darwin && !windows

package hotkey

import (
	"context"
	"errors"
)

func listen(context.Context, []Binding) error {
	return errors.New("hotkey: unsupported platform")
}
