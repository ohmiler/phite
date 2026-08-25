// Package portable defines path-component invariants shared by artifact
// producers and consumers across supported operating systems.
package portable

import (
	"regexp"
	"strings"
)

var namePattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9._-]*[A-Za-z0-9])?$`)

// Name reports whether value is a portable, non-reserved file name or cache
// path component. It deliberately accepts only a conservative ASCII subset.
func Name(value string) bool {
	return namePattern.MatchString(value) && !windowsReservedName(value)
}

func windowsReservedName(value string) bool {
	base := strings.ToUpper(strings.SplitN(value, ".", 2)[0])
	switch base {
	case "CON", "PRN", "AUX", "NUL":
		return true
	}
	if len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) {
		return base[3] >= '1' && base[3] <= '9'
	}
	return false
}
