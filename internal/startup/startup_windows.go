//go:build windows

package startup

import (
	"errors"
	"os"

	"golang.org/x/sys/windows/registry"
)

// HKCU\Software\Microsoft\Windows\CurrentVersion\Run: per-user, no admin
// rights, listed in Task Manager › Startup apps.
const (
	runKey    = `Software\Microsoft\Windows\CurrentVersion\Run`
	valueName = "Spark"
)

func enabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	_, _, err = k.GetStringValue(valueName)
	return err == nil
}

func set(on bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if !on {
		if err := k.DeleteValue(valueName); err != nil && !errors.Is(err, registry.ErrNotExist) {
			return err
		}
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return k.SetStringValue(valueName, `"`+exe+`"`)
}
