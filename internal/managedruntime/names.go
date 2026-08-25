package managedruntime

import (
	"fmt"
	"regexp"
	"strings"
)

var portableFileNamePattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9._-]*[A-Za-z0-9])?$`)

func validRuntimeID(value string) bool {
	return runtimeIDPattern.MatchString(value) &&
		asciiAlphaNumeric(value[len(value)-1]) &&
		!windowsReservedName(value)
}

func validRuntimeFileName(value string) bool {
	return portableFileNamePattern.MatchString(value) && !windowsReservedName(value)
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

func asciiAlphaNumeric(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= '0' && value <= '9'
}

func validateSupport(value string) error {
	if value != "tier1" && value != "experimental" {
		return fmt.Errorf("Runtime Identity support %q must be tier1 or experimental", value)
	}
	return nil
}
