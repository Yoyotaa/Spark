// Package tray shows Spark's icon in the macOS menu bar / Windows
// notification area, with a menu. Spark has no Dock icon nor window frame:
// this menu is how the user finds it, hides it and quits it.
package tray

// Item is a menu entry; an empty Title is a separator.
type Item struct {
	Title  string
	Action func()
}

// Start shows the icon. Actions run on their own goroutine.
func Start(tooltip string, items []Item) error { return start(tooltip, items) }
