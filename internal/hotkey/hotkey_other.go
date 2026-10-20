//go:build !darwin && !windows

package hotkey

import "errors"

func register([]Binding) (*Listener, error) {
	return nil, errors.New("hotkey: unsupported platform")
}

func letterForCode(code string) (Key, bool) {
	if len(code) == 4 && code[:3] == "Key" && code[3] >= 'A' && code[3] <= 'Z' {
		return Key(code[3:]), true
	}
	return "", false
}
