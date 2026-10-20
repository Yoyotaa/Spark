//go:build !darwin && !windows

package audio

import (
	"context"
	"errors"
)

type nopPlayer struct{}

func newPlayer() Player { return nopPlayer{} }

func (nopPlayer) Start(int) error             { return errors.New("audio: no player on this platform") }
func (nopPlayer) Write([]byte)                {}
func (nopPlayer) Drain(context.Context) error { return nil }
func (nopPlayer) Stop()                       {}
