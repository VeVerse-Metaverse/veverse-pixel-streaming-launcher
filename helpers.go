package main

import (
	"dev.hackerman.me/artheon/veverse-shared/executable"
	"fmt"
	"github.com/sirupsen/logrus"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// findEntrypoint searches for the possible entrypoint for the server starting with the root directory
func findEntrypoint(root string) (entrypoint string, err error) {
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			logrus.Errorf("failed to walk directory: %v", err)
			return err
		}

		if d.IsDir() {
			return nil
		}

		if strings.HasSuffix(path, getBinarySuffix()) && strings.Contains(path, "Binaries") && !strings.Contains(path, "Engine") && !strings.Contains(path, "EpicWebHelper") {
			f, err := os.Open(path)
			if err != nil {
				return err
			}

			var isExecutable bool
			isExecutable, err = executable.IsExecutable(f)
			if err != nil {
				err1 := f.Close()
				if err1 != nil {
					log.Printf("failed to close file: %v", err1)
				}
				return err
			}

			err = f.Close()
			if err != nil {
				log.Printf("failed to close file: %v", err)
			}

			if isExecutable {
				entrypoint = path
			}
		}

		return nil
	})

	if entrypoint == "" {
		return "", fmt.Errorf("no entrypoint found")
	}

	return filepath.Abs(entrypoint)
}

// getProjectName extracts the project name from the entrypoint
func getProjectName(entrypoint string) string {
	// Get the entrypoint file name
	base := path.Base(entrypoint)
	if strings.HasSuffix(base, getBinarySuffix()) {
		base = base[:len(base)-len(getBinarySuffix())]
	}
	return base
}
