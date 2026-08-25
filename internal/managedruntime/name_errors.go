package managedruntime

import "fmt"

func invalidRuntimeFileName(field, value string) error {
	return fmt.Errorf("Runtime Manifest %s %q must be a portable file name", field, value)
}
