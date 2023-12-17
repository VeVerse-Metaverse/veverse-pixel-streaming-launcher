package main

import (
	"context"
	"encoding/json"
	"github.com/sirupsen/logrus"
	"log"
	"net/http"
	"os"
	"strconv"
)

var ctx context.Context

func startWebServer(ctx context.Context) {
	http.HandleFunc("/healthcheck", healthCheck)
	http.HandleFunc("/hello", closeSession)

	err := http.ListenAndServe(":8080", nil)
	if err != nil && err != http.ErrServerClosed {
		logrus.Errorf("failed to start web server: %s\n", err.Error())
		err = SetSessionStatus(ctx, session.Id, session.AppId, "closed")
		if err != nil {
			logrus.Errorf("failed close session: %s\n", err.Error())
		}

		os.Exit(1)
		return
	}
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	sess, err := GetSessionData(ctx, session.Id)
	if err != nil {
		logrus.Errorf("failed to get session data: %s\n", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
	}

	t := r.URL.Query().Get("totalCheck")

	var totalCheck int
	totalCheck, err = strconv.Atoi(t)
	if err != nil {
		logrus.Errorf("failed to convert totalCheck to int: %s\n", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
	}

	if sess.Status == "running" && totalCheck >= 20 {
		err = SetSessionStatus(ctx, session.Id, session.AppId, "closed")
		if err != nil {
			logrus.Errorf("failed close session: %s\n", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
		}

		return
	} else {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		resp := make(map[string]string)
		resp["sessionStatus"] = sess.Status
		var jsonResp []byte
		jsonResp, err = json.Marshal(resp)
		if err != nil {
			log.Fatalf("Error happened in JSON marshal. Err: %s", err)
		}

		w.Write(jsonResp)
		return
	}
}

func closeSession(w http.ResponseWriter, r *http.Request) {
	err := SetSessionStatus(ctx, session.Id, session.AppId, "closed")
	if err != nil {
		logrus.Errorf("failed close session: %s\n", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
	}

	// Shutdown the server
	cancel()

	return
}
