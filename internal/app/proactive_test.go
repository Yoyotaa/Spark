package app

import "testing"

func TestIsSilence(t *testing.T) {
	for s, want := range map[string]bool{
		"RAS": true, "ras.": true, " RAS ": true, "« RAS »": true, "": true,
		"RAS rien à signaler":                           true,
		"Attention, le baron est en train d'être pris.": false,
		"Rassemble ton équipe au dragon.":               false,
	} {
		if got := isSilence(s); got != want {
			t.Errorf("isSilence(%q) = %v, want %v", s, got, want)
		}
	}
}
