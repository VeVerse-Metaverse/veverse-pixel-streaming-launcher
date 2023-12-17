// Package utils provides utility functions for the launcher.
package utils

import (
	"archive/zip"
	"dev.hackerman.me/artheon/veverse-shared/executable"
	"fmt"
	"github.com/gofrs/uuid"
	"github.com/sirupsen/logrus"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"veverse-pixel-streaming-launcher/config"
)

// ExtractArchive extracts the given archive to the given destination path.
func ExtractArchive(archivePath string, destinationPath string) error {
	logrus.Printf("extracting archive %s to %s", archivePath, destinationPath)

	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open archive: %w", err)
	}

	defer func(r *zip.ReadCloser) {
		if err1 := r.Close(); err1 != nil {
			logrus.Errorf("failed to close archive: %s", err1)
		}
	}(r)

	err = os.MkdirAll(destinationPath, 0755)
	if err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	extractAndWriteFile := func(f *zip.File) error {
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("failed to open file in archive: %w", err)
		}

		defer func(rc io.ReadCloser) {
			if err1 := rc.Close(); err1 != nil {
				logrus.Errorf("failed to close archive file: %s", err1)
			}
		}(rc)

		path := filepath.Join(destinationPath, f.Name)

		if !strings.HasPrefix(path, filepath.Clean(destinationPath)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", path)
		}

		if f.FileInfo().IsDir() {
			err = os.MkdirAll(path, f.Mode())
			if err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		} else {
			err = os.MkdirAll(filepath.Dir(path), f.Mode())
			if err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}

			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return fmt.Errorf("failed to open file: %w", err)
			}
			defer func(f *os.File) {
				if err1 := f.Close(); err1 != nil {
					logrus.Errorf("failed to close file: %s", err1)
				}
			}(f)

			_, err = io.Copy(f, rc)
			if err != nil {
				return fmt.Errorf("failed to write file: %w", err)
			}
		}

		return nil
	}

	for _, f := range r.File {
		err = extractAndWriteFile(f)
		if err != nil {
			return fmt.Errorf("failed to extract file: %w", err)
		}
	}

	return nil
}

// getAppExecutableById returns the executable path for the given app id located in the given directory
func getAppExecutableById(dir string, id uuid.UUID) (string, error) {
	path := filepath.Join(dir, id.String())
	if //goland:noinspection GoBoolExpressions
	runtime.GOOS == "windows" {
		path = path + ".exe"
	}
	appExe, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	fi, err := os.Stat(appExe)
	if err != nil {
		return "", err
	}

	if fi.IsDir() {
		return "", fmt.Errorf("app executable is a directory")
	}

	return appExe, nil
}

// getAppExecutable returns the executable path for the given app located in the given directory
func getAppExecutableByName(dir string, name string) (string, error) {
	path := filepath.Join(dir, name)
	if //goland:noinspection GoBoolExpressions
	runtime.GOOS == "windows" {
		path = path + ".exe"
	}
	appExe, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	fi, err := os.Stat(appExe)
	if err != nil {
		return "", err
	}

	if fi.IsDir() {
		return "", fmt.Errorf("app executable is a directory")
	}

	return appExe, nil
}

// getAppExecutable returns the executable path for the given app located in the given directory
func getAppExecutableByGenericName(dir string) (string, error) {
	path := filepath.Join(dir, "Metaverse")
	if //goland:noinspection GoBoolExpressions
	runtime.GOOS == "windows" {
		path = path + ".exe"
	}
	appExe, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	fi, err := os.Stat(appExe)
	if err != nil {
		return "", err
	}

	if fi.IsDir() {
		return "", fmt.Errorf("app executable is a directory")
	}

	return appExe, nil
}

// FindAppExecutable returns the executable path for the given app located in the given directory
func FindAppExecutable(id uuid.UUID, name string) (string, error) {
	var err error

	workDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	applicationsDir := filepath.Join(workDir, config.AppDir, id.String())

	_, err = os.Stat(applicationsDir)
	if err != nil {
		if os.IsNotExist(err) {
			logrus.Warningf("applications directory does not exist: %s", applicationsDir)
			return "", err
		}
		logrus.Warningf("failed to stat applications directory: %s", applicationsDir)
		return "", fmt.Errorf("failed to stat app directory: %w", err)
	}

	var appPath string
	if !id.IsNil() {
		appPath, err = getAppExecutableById(applicationsDir, id)
		if err == nil {
			return appPath, nil
		}
	}

	if name != "" {
		appPath, err = getAppExecutableByName(applicationsDir, name)
		if err == nil {
			return appPath, nil
		}
	}

	appPath, err = getAppExecutableByGenericName(applicationsDir)
	if err == nil {
		return appPath, nil
	}

	err = filepath.WalkDir(applicationsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		if !info.Mode().IsRegular() {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}

		defer func() {
			err = f.Close()
			if err != nil {
				logrus.Errorf("failed to close file: %s", err)
			}
		}()

		var isExecutable bool
		isExecutable, err = executable.IsExecutable(f)
		if err != nil {
			return err
		}

		if isExecutable {
			appPath = path
			return io.EOF
		}

		appPath = path

		return nil
	})

	if err != nil && err != io.EOF {
		return "", fmt.Errorf("failed to find app executable: %w", err)
	}

	if appPath == "" {
		return "", fmt.Errorf("failed to find app executable")
	}

	appPath, err = filepath.Abs(appPath)
	if err != nil {
		return "", fmt.Errorf("failed to find app executable: %w", err)
	}

	return appPath, nil
}

// ClearUserData clears the user data such as the API tokens, sessions, etc.
func ClearUserData() error {
	logrus.Println("clearing user data")

	// Get the user data directory.
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		logrus.Errorf("failed to get user home directory: %s", err)
	}

	// Get the project directory.
	executablePath, err := os.Executable()
	if err != nil {
		logrus.Errorf("failed to get executable path: %s", err)
	}
	projectDir := filepath.Dir(executablePath) + filepath.FromSlash("/Metaverse")

	if runtime.GOOS == "windows" {
		userDataDir := os.Getenv("LOCALAPPDATA") + filepath.FromSlash("/Metaverse/Saved")
		if _, err = os.Stat(userDataDir); os.IsNotExist(err) {
			userDataDir = userHomeDir + filepath.FromSlash("/AppData/Local/Metaverse/Saved")
		}

		logsDir := userDataDir + filepath.FromSlash("/Logs")
		if _, err = os.Stat(logsDir); !os.IsNotExist(err) {
			err = os.RemoveAll(logsDir)
			if err != nil {
				logrus.Errorf("failed to remove user logs directory: %s", err)
			}
		}

		sessionPath := userDataDir + filepath.FromSlash("/.session.bin")
		if _, err = os.Stat(sessionPath); !os.IsNotExist(err) {
			err = os.Remove(sessionPath)
			if err != nil {
				logrus.Errorf("failed to remove user session: %s", err)
			}
		}

		apiTokenPath := userDataDir + filepath.FromSlash("/ApiToken.dat")
		if _, err = os.Stat(apiTokenPath); !os.IsNotExist(err) {
			err = os.Remove(apiTokenPath)
			if err != nil {
				logrus.Errorf("failed to remove user api token: %s", err)
			}
		}

		projectLogsDir := projectDir + filepath.FromSlash("/Saved/Logs")
		if _, err = os.Stat(projectLogsDir); !os.IsNotExist(err) {
			err = os.RemoveAll(projectLogsDir)
			if err != nil {
				logrus.Errorf("failed to remove project logs directory: %s", err)
			}
		}

		projectSessionPath := projectDir + filepath.FromSlash("/Saved/.session.bin")
		if _, err = os.Stat(projectSessionPath); !os.IsNotExist(err) {
			err = os.Remove(projectSessionPath)
			if err != nil {
				logrus.Errorf("failed to remove project session: %s", err)
			}
		}

		projectApiTokenPath := projectDir + filepath.FromSlash("/Saved/ApiToken.dat")
		if _, err = os.Stat(projectApiTokenPath); !os.IsNotExist(err) {
			err = os.Remove(projectApiTokenPath)
			if err != nil {
				logrus.Errorf("failed to remove project api token: %s", err)
			}
		}
	} else if runtime.GOOS == "darwin" {
		// warning: this is not tested
		userDataDir := userHomeDir + "/Library/Application Support/Metaverse/Saved"
		if _, err = os.Stat(userDataDir); os.IsNotExist(err) {
			userDataDir = userHomeDir + "/.config/Metaverse/Saved"
		}

		logsDir := userDataDir + filepath.FromSlash("/Logs")
		if _, err = os.Stat(logsDir); !os.IsNotExist(err) {
			err = os.RemoveAll(logsDir)
			if err != nil {
				logrus.Errorf("failed to remove user logs directory: %s", err)
			}
		}

		sessionPath := userDataDir + filepath.FromSlash("/.session.bin")
		if _, err = os.Stat(sessionPath); !os.IsNotExist(err) {
			err = os.Remove(sessionPath)
			if err != nil {
				logrus.Errorf("failed to remove user session: %s", err)
			}
		}

		apiTokenPath := userDataDir + filepath.FromSlash("/ApiToken.dat")
		if _, err = os.Stat(apiTokenPath); !os.IsNotExist(err) {
			err = os.Remove(apiTokenPath)
			if err != nil {
				logrus.Errorf("failed to remove user api token: %s", err)
			}
		}

		projectLogsDir := projectDir + filepath.FromSlash("/Saved/Logs")
		if _, err = os.Stat(projectLogsDir); !os.IsNotExist(err) {
			err = os.RemoveAll(projectLogsDir)
			if err != nil {
				logrus.Errorf("failed to remove project logs directory: %s", err)
			}
		}

		projectSessionPath := projectDir + filepath.FromSlash("/Saved/.session.bin")
		if _, err = os.Stat(projectSessionPath); !os.IsNotExist(err) {
			err = os.Remove(projectSessionPath)
			if err != nil {
				logrus.Errorf("failed to remove project session: %s", err)
			}
		}

		projectApiTokenPath := projectDir + filepath.FromSlash("/Saved/ApiToken.dat")
		if _, err = os.Stat(projectApiTokenPath); !os.IsNotExist(err) {
			err = os.Remove(projectApiTokenPath)
			if err != nil {
				logrus.Errorf("failed to remove project api token: %s", err)
			}
		}
	} else if runtime.GOOS == "linux" {
		// warning: this is not tested
		userDataDir := userHomeDir + filepath.FromSlash("/.config/Metaverse/Saved")
		if _, err = os.Stat(userDataDir); os.IsNotExist(err) {
			userDataDir = userHomeDir + filepath.FromSlash("/.local/share/Metaverse/Saved")
		}

		logsDir := userDataDir + filepath.FromSlash("/Logs")
		if _, err = os.Stat(logsDir); !os.IsNotExist(err) {
			err = os.RemoveAll(logsDir)
			if err != nil {
				logrus.Errorf("failed to remove user logs directory: %s", err)
			}
		}

		sessionPath := userDataDir + filepath.FromSlash("/.session.bin")
		if _, err = os.Stat(sessionPath); !os.IsNotExist(err) {
			err = os.Remove(sessionPath)
			if err != nil {
				logrus.Errorf("failed to remove user session: %s", err)
			}
		}

		apiTokenPath := userDataDir + filepath.FromSlash("/ApiToken.dat")
		if _, err = os.Stat(apiTokenPath); !os.IsNotExist(err) {
			err = os.Remove(apiTokenPath)
			if err != nil {
				logrus.Errorf("failed to remove user api token: %s", err)
			}
		}

		projectLogsDir := projectDir + filepath.FromSlash("/Saved/Logs")
		if _, err = os.Stat(projectLogsDir); !os.IsNotExist(err) {
			err = os.RemoveAll(projectLogsDir)
			if err != nil {
				logrus.Errorf("failed to remove project logs directory: %s", err)
			}
		}

		projectSessionPath := projectDir + filepath.FromSlash("/Saved/.session.bin")
		if _, err = os.Stat(projectSessionPath); !os.IsNotExist(err) {
			err = os.Remove(projectSessionPath)
			if err != nil {
				logrus.Errorf("failed to remove project session: %s", err)
			}
		}

		projectApiTokenPath := projectDir + filepath.FromSlash("/Saved/ApiToken.dat")
		if _, err = os.Stat(projectApiTokenPath); !os.IsNotExist(err) {
			err = os.Remove(projectApiTokenPath)
			if err != nil {
				logrus.Errorf("failed to remove project api token: %s", err)
			}
		}
	}

	return err
}
