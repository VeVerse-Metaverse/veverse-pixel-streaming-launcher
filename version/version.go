// Package version provides functions for reading and writing the version of a launcher or app release.
package version

import (
	"encoding/binary"
	"fmt"
	"github.com/Masterminds/semver"
	"os"
	"path/filepath"
)

// ReadVersion reads the version of a launcher or app release from the .version file in the given directory.
func ReadVersion(dir string) (*semver.Version, error) {
	versionFile := filepath.Join(dir, ".version")

	if _, err := os.Stat(versionFile); os.IsNotExist(err) {
		return &semver.Version{}, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to check if version file exists: %w", err)
	}

	versionBytes, err := os.ReadFile(versionFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read version from file: %w", err)
	}

	versionMajor := binary.LittleEndian.Uint32(versionBytes[0:4])
	versionMinor := binary.LittleEndian.Uint32(versionBytes[4:8])
	versionPatch := binary.LittleEndian.Uint32(versionBytes[8:12])

	version, err := semver.NewVersion(fmt.Sprintf("%d.%d.%d", versionMajor, versionMinor, versionPatch))
	if err != nil {
		return nil, fmt.Errorf("failed to parse version: %w", err)
	}

	return version, nil
}

// WriteVersion writes the version of a launcher or app release to the .version file in the given directory.
func WriteVersion(dir string, version *semver.Version) error {
	versionFile := filepath.Join(dir, ".version")

	_, err := os.Stat(versionFile)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to check if version file exists: %w", err)
		}

		_, err = os.Create(versionFile)
		if err != nil {
			return fmt.Errorf("failed to create version file: %w", err)
		}
	} else {
		err = os.Remove(versionFile)
		if err != nil {
			return fmt.Errorf("failed to remove version file: %w", err)
		}
	}

	versionBytes := make([]byte, 12)
	binary.LittleEndian.PutUint32(versionBytes[0:4], uint32(version.Major()))
	binary.LittleEndian.PutUint32(versionBytes[4:8], uint32(version.Minor()))
	binary.LittleEndian.PutUint32(versionBytes[8:12], uint32(version.Patch()))

	err = os.WriteFile(versionFile, versionBytes, 0644)
	if err != nil {
		return fmt.Errorf("failed to write version to file: %w", err)
	}

	return nil
}
