package api

import (
	"bytes"
	"context"
	vContext "dev.hackerman.me/artheon/veverse-shared/context"
	"dev.hackerman.me/artheon/veverse-shared/model"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	glConfig "veverse-pixel-streaming-launcher/config"
)

func Login(ctx context.Context, email string, password string) (context.Context, error) {
	var (
		requestBody []byte
		err         error
	)

	requestBody, err = json.Marshal(map[string]string{
		"email":    email,
		"password": password,
	})

	if err != nil {
		log.Fatalln(err)
	}

	url := fmt.Sprintf("%s/auth/login", glConfig.Api2Url)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		log.Fatalln(err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return ctx, err
	}

	defer func(body io.ReadCloser) {
		err := body.Close()
		if err != nil {
			log.Printf("error closing http response body: %s\n", err.Error())
		}
	}(resp.Body)

	if resp.StatusCode >= 400 {
		return ctx, fmt.Errorf("failed to login to %s, status code: %d, error: %s", url, resp.StatusCode, err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ctx, err
	}

	var v model.Wrapper[string]
	if err = json.Unmarshal(body, &v); err != nil {
		return ctx, err
	}

	if v.Status == "error" {
		return ctx, fmt.Errorf("authentication error %d: %s\n", resp.StatusCode, v.Message)
	} else if v.Status == "ok" {
		return context.WithValue(ctx, vContext.Token, v.Payload), nil
	}

	return ctx, fmt.Errorf(v.Message)
}
