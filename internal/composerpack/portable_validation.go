package composerpack

import (
	"errors"
	"fmt"

	"github.com/ohmiler/phite/internal/portable"
)

func validatePortableRecipeIdentity(configuration recipe) error {
	if !portable.Name(configuration.Version) {
		return errors.New("Composer recipe version must be a portable path component")
	}
	for field, value := range map[string]string{
		"artifact.name": configuration.Artifact.Name,
		"notices.name":  configuration.Notices.Name,
	} {
		if !portable.Name(value) {
			return fmt.Errorf("Composer recipe %s must be a portable file name", field)
		}
	}
	return nil
}
