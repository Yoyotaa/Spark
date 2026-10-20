package overlay

// Rect is an interactive region of the UI, in CSS pixels relative to the
// top-left corner of the overlay window. The frontend reports these regions
// (chat panel, buttons...) so that the native layer only captures the mouse
// when the cursor is above one of them.
type Rect struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

func (r Rect) contains(x, y float64) bool {
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

// hit reports whether (x, y) falls inside any region. Linear scan on purpose:
// the overlay exposes a handful of regions, which beats any spatial index.
func hit(regions []Rect, x, y float64) bool {
	for i := range regions {
		if regions[i].contains(x, y) {
			return true
		}
	}
	return false
}
