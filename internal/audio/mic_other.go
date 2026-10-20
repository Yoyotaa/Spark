//go:build !darwin && !windows

package audio

import "errors"

type nopInput struct{}

func NewInput(InputEvents, Transcriber) Input { return nopInput{} }

func (nopInput) StartMic() error { return errors.New("micro non pris en charge sur cette plateforme") }
func (nopInput) StopMic()        {}
func (nopInput) Begin(string) error {
	return errors.New("micro non pris en charge sur cette plateforme")
}
func (nopInput) End()       {}
func (nopInput) Cancel()    {}
func (nopInput) Live() bool { return false }

func MicPermission() Permission          { return PermissionDenied }
func SpeechPermission() Permission       { return PermissionDenied }
func RequestPermissions(done func(bool)) { done(false) }
func PlayCue(bool)                       {}
