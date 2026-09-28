package model

// Process colors of a four-color press, in the unit order corrections are
// applied: cyan, magenta, yellow, black.
const (
	Cyan    = "cyan"
	Magenta = "magenta"
	Yellow  = "yellow"
	Black   = "black"
)

// ProcessColors returns the four process colors in deterministic unit order.
func ProcessColors() []string {
	return []string{Cyan, Magenta, Yellow, Black}
}

// IsProcessColor reports whether color is one of the four process colors.
func IsProcessColor(color string) bool {
	for _, known := range ProcessColors() {
		if color == known {
			return true
		}
	}
	return false
}
