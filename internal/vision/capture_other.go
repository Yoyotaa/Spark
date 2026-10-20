//go:build !darwin && !windows

package vision

import (
	"context"
	"errors"
)

type nopPlatform struct{}

func newPlatform() platform { return nopPlatform{} }

func hasPermission() bool { return false }

func requestPermission() {}

func (nopPlatform) captureJPEG(_ context.Context, _ Options, dst []byte) ([]byte, int, int, error) {
	return dst, 0, 0, errors.New("vision: unsupported platform")
}
