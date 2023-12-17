// Package api contains all the API calls to the Veverse API.
package api

import (
	"context"
	sm "dev.hackerman.me/artheon/veverse-shared/model"
	vUnreal "dev.hackerman.me/artheon/veverse-shared/unreal"
	"encoding/json"
	"fmt"
	"github.com/Masterminds/semver"
	"github.com/gofrs/uuid"
	"github.com/sirupsen/logrus"
	"io"
	"net/http"
	"os"
	_ "veverse-pixel-streaming-launcher/config"
)

var api2Root string

func init() {
	api2Root = os.Getenv("VE_API2_ROOT_URL")

	if api2Root == "" {
		logrus.Fatalf("invalid VE_API2_ROOT_URL env\n")
	}
}

// GetLatestReleaseV2 returns the latest release metadata for the given app id.
func GetLatestReleaseV2(ctx context.Context, id uuid.UUID) (*sm.ReleaseV2, error) {
	if id.IsNil() {
		return nil, fmt.Errorf("app id is not set")
	}

	url := fmt.Sprintf("%s/apps/public/%s?platform=%s", api2Root, id, vUnreal.GetPlatformName())

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create a HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send a HTTP GET request: %w", err)
	}

	defer func(body io.ReadCloser) {
		err := body.Close()
		if err != nil {
			logrus.Errorf("error closing http response body: %v", err)
		}
	}(resp.Body)

	if resp.StatusCode >= 400 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return nil, fmt.Errorf("failed to get launcher metadata from %s, status code: %d, content: %s", url, resp.StatusCode, body)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var v sm.Wrapper[sm.AppV2]
	if err = json.Unmarshal(body, &v); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	var latestVersion *semver.Version
	var latestRelease *sm.ReleaseV2
	for _, release := range v.Payload.Releases.Entities {
		if latestVersion == nil {
			latestRelease = &release
			latestVersion, err = semver.NewVersion(release.Version)
			if err != nil {
				logrus.Errorf("failed to parse version: %s", err)
				return nil, fmt.Errorf("failed to parse version: %w", err)
			}
		} else {
			releaseVersion, err := semver.NewVersion(release.Version)
			if err != nil {
				logrus.Errorf("failed to parse semver: %s", err)
				return nil, fmt.Errorf("failed to parse semver: %w", err)
			}

			if latestVersion.GreaterThan(releaseVersion) {
				latestVersion = releaseVersion
				latestRelease = &release
			}
		}
	}

	if latestVersion == nil || latestRelease == nil {
		logrus.Errorf("failed to find latest version")
		return nil, fmt.Errorf("failed to find latest version")
	}

	return latestRelease, err
}
