package main

import (
	"context"
	sm "dev.hackerman.me/artheon/veverse-shared/model"
	"fmt"
	"github.com/Masterminds/semver"
	"github.com/gofrs/uuid"
	"github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"veverse-pixel-streaming-launcher/config"
	"veverse-pixel-streaming-launcher/http"
	"veverse-pixel-streaming-launcher/utils"
	"veverse-pixel-streaming-launcher/version"
)

func installAppReleaseArchive(ctx context.Context, appId uuid.UUID, release sm.ReleaseV2) error {
	logrus.Debugf("installing app release archive...")

	logrus.Debugf("getting archive file...")
	var archive *sm.File
	for _, file := range release.Files.Entities {
		if file.Type == "release-archive" {
			logrus.Debugf("found archive file %s: %s", file.Id, file.Url)
			archive = &file
			break
		}
	}
	if archive == nil {
		return fmt.Errorf("no archive file found")
	}
	logrus.Debugf("archive file found: %+v", archive)

	logrus.Debugf("getting working directory...")
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}
	logrus.Debugf("working directory: %s", wd)

	tempDownloadPath := filepath.Join(wd, config.TempDir, config.DownloadDir, appId.String(), release.Id.String()+"-"+release.Version)
	logrus.Debugf("temp download path: %s", tempDownloadPath)
	appInstallationPath := filepath.Join(wd, config.AppDir, appId.String(), release.Id.String()+"-"+release.Version)
	logrus.Debugf("app installation path: %s", appInstallationPath)

	counter := http.NewDownloadProgressTracker((uint64)(*archive.Size), func(progress uint64, total uint64) {
		logrus.Printf("downloading file: %d/%d", progress, total)
	})
	logrus.Debugf("downloading file to %s...", tempDownloadPath)
	err = http.DownloadFile(ctx, tempDownloadPath, archive.Url, counter)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	logrus.Debugf("downloaded file to %s", tempDownloadPath)

	logrus.Debugf("extracting archive to %s...", appInstallationPath)
	err = utils.ExtractArchive(tempDownloadPath, appInstallationPath)
	if err != nil {
		return fmt.Errorf("failed to extract archive: %w", err)
	}
	logrus.Debugf("extracted archive to %s", appInstallationPath)

	logrus.Debugf("parsing release version: %s...", release.Version)
	v, err := semver.NewVersion(release.Version)
	if err != nil {
		return fmt.Errorf("failed to parse release version: %w", err)
	}
	logrus.Debugf("parsed release version: %s", v.String())

	logrus.Debugf("writing version to %s...", appInstallationPath)
	err = version.WriteVersion(appInstallationPath, v)
	if err != nil {
		return fmt.Errorf("failed to write version: %w", err)
	}
	logrus.Debugf("wrote version to %s", appInstallationPath)

	logrus.Debugf("removing temporary download directory %s...", tempDownloadPath)
	err = os.RemoveAll(tempDownloadPath)
	if err != nil {
		return fmt.Errorf("failed to remove temporary download directory: %w", err)
	}
	logrus.Debugf("removed temporary download directory %s", tempDownloadPath)

	return nil
}

func installAppRelease(ctx context.Context, appId uuid.UUID, release sm.ReleaseV2) error {
	logrus.Debugf("installing app release: %+v", release)

	var files []*sm.File
	for _, file := range release.Files.Entities {
		if file.Type == "release" {
			logrus.Debugf("found release file %s: %s", file.Id, file.Url)
			files = append(files, &file)
		}
	}

	if len(files) == 0 {
		return fmt.Errorf("no release files found")
	}

	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	tempDownloadPath := filepath.Join(wd, config.TempDir, config.DownloadDir, appId.String(), release.Id.String()+"-"+release.Version)
	appInstallationPath := filepath.Join(wd, config.AppDir, appId.String(), release.Id.String()+"-"+release.Version)

	var totalProgress uint64 = 0
	var totalSize uint64 = 0

	// calculate total size for all files
	for _, file := range files {
		totalSize += uint64(*file.Size)
	}

	logrus.Debugf("total size: %d", totalSize)

	for _, file := range files {
		counter := http.NewDownloadProgressTracker(totalSize, func(progress uint64, total uint64) {
			// accumulate progress for all files and report it to the frontend as total progress
			totalProgress += progress
		})
		// download next file
		err = http.DownloadFile(ctx, tempDownloadPath, file.Url, counter)
		if err != nil {
			logrus.Errorf("failed to download file: %s", err.Error())
		}
	}

	for _, file := range files {
		if file.OriginalPath == nil {
			logrus.Errorf("file %s has no original path", file.Id)
			continue
		}
		err = os.Rename(filepath.Join(tempDownloadPath, *file.OriginalPath), filepath.Join(appInstallationPath, *file.OriginalPath))
		if err != nil {
			logrus.Errorf("failed to move file: %s", err.Error())
		}
	}

	v, err := semver.NewVersion(release.Version)
	if err != nil {
		return fmt.Errorf("failed to parse release version: %w", err)
	}

	err = version.WriteVersion(appInstallationPath, v)
	if err != nil {
		return fmt.Errorf("failed to write version: %w", err)
	}

	err = os.RemoveAll(tempDownloadPath)
	if err != nil {
		return fmt.Errorf("failed to remove temporary download directory: %w", err)
	}

	return nil
}
