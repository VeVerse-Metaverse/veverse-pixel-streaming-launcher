package main

import (
	"bytes"
	"context"
	"dev.hackerman.me/artheon/veverse-shared/executable"
	sm "dev.hackerman.me/artheon/veverse-shared/model"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gofrs/uuid"
	"github.com/sirupsen/logrus"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
)

func init() {
	api2Root = os.Getenv("VE_API2_ROOT_URL")

	if api2Root == "" {
		logrus.Fatalf("invalid VE_API2_ROOT_URL env\n")
	}
}

// login authenticates user with the API
func login() (string, error) {
	var (
		requestBody []byte
		err         error
	)

	requestBody, err = json.Marshal(map[string]string{
		"email":    os.Getenv("USER_EMAIL"),
		"password": os.Getenv("USER_PASSWORD"),
	})

	if err != nil {
		log.Fatalln(err)
	}

	url := fmt.Sprintf("%s/auth/login", api2Root)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		log.Fatalln(err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf("failed to close response body: %v", err)
		}
	}(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var v map[string]string
	if err = json.Unmarshal(body, &v); err != nil {
		return "", err
	}

	if v["status"] == "error" {
		return "", errors.New(fmt.Sprintf("authentication error %d: %s\n", resp.StatusCode, v["message"]))
	} else if v["status"] == "ok" {
		return v["data"], nil
	}

	return "", errors.New(v["message"])
}

// downloadFile downloads file to the filepath from url
func downloadFile(filepath string, url string, size int64) (err error) {
	// Check if file exists
	stat, err := os.Stat(filepath)
	if err == nil {
		if size > 0 && stat.Size() == size {
			log.Printf("skipping, file exists: %s, size matches: %d", filepath, size)
			return nil
		}
	}

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to send a HTTP GET request: %s\n", err.Error())
	}
	defer func(body io.ReadCloser) {
		err := body.Close()
		if err != nil {
			log.Printf("failed to close response body: %v", err)
		}
	}(resp.Body)

	// Check server response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download file %s to %s: bad status: %s\n", url, filepath, resp.Status)
	}

	// Create the dir
	dir := path.Dir(filepath)
	err = os.MkdirAll(dir, 0750)
	if err != nil {
		return fmt.Errorf("failed to create a directory %s: %s\n", dir, err.Error())
	}

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("failed to create a file downloaded %s to %s: %s\n", url, filepath, err.Error())
	}
	defer func(out *os.File) {
		err := out.Close()
		if err != nil {
			log.Printf("failed to close file: %v", err)
		}
	}(out)

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write a file downloaded %s to %s: %s\n", url, filepath, err.Error())
	}

	// Change a file mode for known binaries to make them executable
	for s, b := range binarySuffixes {
		if b && strings.HasSuffix(filepath, s) {
			err = os.Chmod(filepath, 0755)
			if err != nil {
				log.Printf("failed to change file mode for %s: %s\n", filepath, err.Error())
			}
		} else {
			f, err := os.Open(filepath)
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
				err = os.Chmod(filepath, 0755)
				if err != nil {
					log.Printf("failed to change file mode for %s: %s\n", filepath, err.Error())
				}
			}
		}
	}

	return nil
}

func GetPendingSession(ctx context.Context) (session *sm.PixelStreamingSessionData, err error) {
	var (
		req  *http.Request
		resp *http.Response
		body []byte
	)

	url := fmt.Sprintf("%s/pixelstreaming/session/pending", api2Root)
	req, err = http.NewRequest("GET", url, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", ctx.Value("token")))

	client := &http.Client{}
	resp, err = client.Do(req)
	if err != nil {
		return nil, err
	}

	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			log.Printf("failed to close response body: %v", err)
		}
	}(resp.Body)

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	v := struct {
		Status  string
		Message string
		Data    *sm.PixelStreamingSessionData
	}{}

	if err = json.Unmarshal(body, &v); err != nil {
		return nil, err
	}

	if v.Status == "error" {
		return nil, errors.New(fmt.Sprintf("authentication error %d: %s\n", resp.StatusCode, v.Message))
	} else if v.Status == "ok" {
		session = v.Data
		return session, nil
	}

	return &sm.PixelStreamingSessionData{}, nil
}

func GetSessionData(ctx context.Context, sessionId *uuid.UUID) (session *sm.PixelStreamingSessionData, err error) {
	var (
		req  *http.Request
		resp *http.Response
		body []byte
	)

	url := fmt.Sprintf("%s/pixelstreaming/session/%s", api2Root, sessionId)
	req, err = http.NewRequest("GET", url, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", ctx.Value("token")))

	client := &http.Client{}
	resp, err = client.Do(req)
	if err != nil {
		return nil, err
	}

	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			log.Printf("failed to close response body: %v", err)
		}
	}(resp.Body)

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	v := struct {
		Status  string
		Message string
		Data    *sm.PixelStreamingSessionData
	}{}

	if err = json.Unmarshal(body, &v); err != nil {
		return nil, err
	}

	if v.Status == "error" {
		return nil, errors.New(fmt.Sprintf("get session data error %d: %s\n", resp.StatusCode, v.Message))
	} else if v.Status == "ok" {
		session = v.Data
		return session, nil
	}

	return &sm.PixelStreamingSessionData{}, nil
}

func SetInstanceStatus(ctx context.Context, instanceId string, status string) (err error) {
	var (
		req  *http.Request
		resp *http.Response
		body []byte
	)

	body, err = json.Marshal(map[string]interface{}{
		"instanceId": instanceId,
		"status":     status,
	})
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/pixelstreaming/instance/status", api2Root)
	req, err = http.NewRequest(http.MethodPut, url, bytes.NewBuffer(body))

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", ctx.Value("token")))

	client := &http.Client{}
	resp, err = client.Do(req)
	if err != nil {
		return err
	}

	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			log.Printf("failed to close response body: %v", err)
		}
	}(resp.Body)

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	v := struct {
		Status  string
		Message string
	}{}

	if err = json.Unmarshal(body, &v); err != nil {
		return err
	}

	if v.Status == "error" {
		return errors.New(fmt.Sprintf("authentication error %d: %s\n", resp.StatusCode, v.Message))
	} else if v.Status == "ok" {
		return nil
	}

	return nil
}

func SetSessionStatus(ctx context.Context, id *uuid.UUID, appId *uuid.UUID, status string) (err error) {
	var (
		req  *http.Request
		resp *http.Response
		body []byte
	)

	body, err = json.Marshal(map[string]interface{}{
		"appId":  appId,
		"status": status,
	})
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/pixelstreaming/session/%s", api2Root, id)
	req, err = http.NewRequest(http.MethodPut, url, bytes.NewBuffer(body))

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", ctx.Value("token")))

	client := &http.Client{}
	resp, err = client.Do(req)
	if err != nil {
		return err
	}

	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			log.Printf("failed to close response body: %v", err)
		}
	}(resp.Body)

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	v := struct {
		Status  string
		Message string
		Data    *sm.PixelStreamingSessionData
	}{}

	if err = json.Unmarshal(body, &v); err != nil {
		return err
	}

	if v.Status == "error" {
		return errors.New(fmt.Sprintf("authentication error %d: %s\n", resp.StatusCode, v.Message))
	} else if v.Status == "ok" {
		return nil
	}

	return nil
}
