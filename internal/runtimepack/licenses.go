package runtimepack

import (
	"encoding/csv"
	"fmt"
	"path/filepath"
	"strings"
)

func inventoryPackageNames(data []byte) []string {
	rows, err := csv.NewReader(strings.NewReader(string(data))).ReadAll()
	if err != nil {
		return nil
	}

	packages := make([]string, 0, len(rows))
	for _, row := range rows {
		packages = append(packages, row[0])
	}
	return packages
}

func validateBundledLicenses(packages []string, entries []noticeEntry) error {
	for _, packagePath := range packages {
		prefix := "licenses/" + strings.Trim(filepath.ToSlash(packagePath), "/") + "/"
		found := false
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name, prefix) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("license inventory has no bundled license files for %s", packagePath)
		}
	}
	return nil
}
