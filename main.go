/*
1. The launcher should start automatically after starting/restarting the instance.
2. After starting, the launcher finds a pending session, get the session_id, instance_id, instance_type, app_id and world_id, and sets the session status to "starting."
3. Launcher clears all user data.
4. It downloads the necessary app, installs and launches it. Switches the session status to "Running."
5. It periodically checks the status, if the status is "Closed" it closes the app.
*/

package main

import (
	"context"
	sl "dev.hackerman.me/artheon/veverse-shared/log"
	sm "dev.hackerman.me/artheon/veverse-shared/model"
	"flag"
	"fmt"
	"github.com/gofrs/uuid"
	"github.com/sirupsen/logrus"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"veverse-pixel-streaming-launcher/api"
	"veverse-pixel-streaming-launcher/config"
	"veverse-pixel-streaming-launcher/database"
)

var (
	pEnvironment string // environment (development, test or production), will use a corresponding API to get jobs for processing
	api2Root     string
	instanceId   string

	NewSessionCheckTime = time.Duration(30) * time.Second
	session             *sm.PixelStreamingSessionData
	isAppLaunch         bool
	latestRelease       *sm.ReleaseV2
	cmd                 *exec.Cmd
	cancel              context.CancelFunc
)

func init() {
	flag.StringVar(&pEnvironment, "env", "", "Environment: dev, test or prod")
	flag.Parse()

	//region Parse environment variables

	if pEnvironment == "" {
		pEnvironment = "test"
	}

	instanceId = os.Getenv("INSTANCE_ID")

	isAppLaunch = false
	session = &sm.PixelStreamingSessionData{}
	//endregion
}

func main() {

	fmt.Println("Welcome to the VeVerse pixel streaming launcher")

	//region Authenticate and get the JWT
	// create context for web server with cancel function
	ctx, cancel = context.WithCancel(context.Background())

	var err error

	ctx, err = database.SetupClickhouse(ctx)
	if err != nil {
		logrus.Errorf("failed to setup clickhouse: %s\n", err.Error())
	}

	var hook logrus.Hook
	hook, err = sl.NewHook(ctx)
	if err != nil {
		logrus.Errorf("failed to setup logrus hook: %s\n", err.Error())
	} else {
		logrus.AddHook(hook)
	}

	token, err := login()
	if err != nil {
		logrus.Errorf("failed to login: %s\n", err.Error())
	} else {
		ctx = context.WithValue(ctx, "token", token)
	}

	//endregion

	err = SetInstanceStatus(ctx, instanceId, "free")

	// start web server for cirrus session management
	go startWebServer(ctx)

	// region check pending session
	for {
		// get pending session
		session, err = GetPendingSession(ctx)
		if err != nil {
			logrus.Errorf("failed to get latest release: %s\n", err.Error())
		}

		if session != nil && session.Id != nil {
			break
		}

		time.Sleep(NewSessionCheckTime)
	}
	// endregion

	//region change session status & launch app
	for {
		if !isAppLaunch && session.Id != nil {
			// update session status to starting
			err = SetSessionStatus(ctx, session.Id, session.AppId, "starting")
			if err != nil {
				log.Fatalf("failed to set session status to starting: %s\n", err.Error())
			}

			// launch app
			latestRelease, err = api.GetLatestReleaseV2(ctx, *session.AppId)
			if err != nil || latestRelease == nil {
				log.Fatalf("failed to get the latest release: %s\n", err.Error())
			}

			//region Download binaries

			if latestRelease.Files == nil || latestRelease.Files.Entities == nil || len(latestRelease.Files.Entities) == 0 {
				log.Fatalf("no files in the release\n")
			}

			if latestRelease.Archive {
				err = installAppReleaseArchive(ctx, *session.AppId, *latestRelease)
				if err != nil {
					log.Fatalf("failed to download the archive: %s\n", err.Error())
				}
			} else {
				err = installAppRelease(ctx, *session.AppId, *latestRelease)
				if err != nil {
					log.Fatalf("failed to download the files: %s\n", err.Error())
				}
			}

			// update session status to running
			err = SetSessionStatus(ctx, session.Id, session.AppId, "running")
			if err != nil {
				log.Fatalf("failed to set session status to starting: %s\n", err.Error())
			}

			isAppLaunch = true

			runApp(ctx, *session.AppId, latestRelease)
		} else if session.Id != nil {
			break
		}

		time.Sleep(NewSessionCheckTime)
	}
	//endregion

	<-ctx.Done()
}

func runApp(ctx context.Context, id uuid.UUID, r *sm.ReleaseV2) {
	//region Entrypoint

	entrypoint, err := findEntrypoint(filepath.Join(config.AppDir, id.String(), r.Id.String()+"-"+r.Version))
	if err != nil || entrypoint == "" {
		log.Fatalf("failed to find an entrypoint: %s\n", err.Error())
	}

	projectName := getProjectName(entrypoint)

	// Get the PROJECT_DIR basing on the entrypoint as "../../../"
	projectDir := path.Dir(path.Dir(path.Dir(path.Dir(entrypoint)))) + "/"
	// Check if we need to normalize the entrypoint to the PROJECT_DIR
	if strings.Count(entrypoint, "/") > 3 {
		// Check if we need to remove excessive path prefix
		if !strings.HasPrefix(entrypoint, projectName) {
			// Normalize entrypoint to the PROJECT_DIR by removing excessive prefix
			entrypoint = strings.Replace(entrypoint, projectDir, "", 1)
		}
	}

	log.Printf("using entrypoint: %s\n", entrypoint)

	//endregion

	//region Command arguments

	// Set the first command line argument as the project name
	var args = []string{"-PixelStreamingIP=127.0.0.1", "-PixelStreamingPort=8888", "-RenderOffScreen", "-ForceRes", "-ResX=1920", "-ResY=1080"}
	// Append additional command line arguments if any of them present
	args = append(args, os.Args[1:]...)

	//endregion

	//region Prepare and run the server command
	cmd = exec.Command(entrypoint, args...)
	cmd.Dir = projectDir // Change the current working directory for the process to the PROJECT_DIR
	cmd.Env = os.Environ()
	rd, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatalf("failed to attach to a application stdout pipe: %s\n", err.Error())
	}

	// Read output from the server process
	go func() {
		b := make([]byte, 2048)
		for {
			nn, err := rd.Read(b)
			if nn > 0 {
				log.Printf("%s", b[:nn])
			}
			if err != nil {
				if err == io.EOF {
					log.Printf("the application process has exited\n")
				} else {
					log.Fatalf("failed to read the application process pipe: %s\n", err.Error())
				}
				return
			}
		}
	}()

	if err := cmd.Start(); err != nil {
		log.Fatalf("cmd.Start() error: %v\n", err)
	}

	if err := cmd.Wait(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			// The program has exited with an exit code != 0
			// This usually means that the server process has crashed
			err = SetSessionStatus(ctx, session.Id, session.AppId, "closed")
			if err != nil {
				log.Fatalf("failed to set session status to starting: %s\n", err.Error())
			}

			if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
				log.Fatalf("application exit code: %v\n", status.ExitStatus())
			}
		} else {
			err = SetSessionStatus(ctx, session.Id, session.AppId, "closed")
			if err != nil {
				log.Fatalf("failed to set session status to starting: %s\n", err.Error())
			}
			log.Fatalf("application exit error: %v\n", err)
		}
	} else {
		log.Printf("application exited normally\n")
		err = SetSessionStatus(ctx, session.Id, session.AppId, "closed")
		if err != nil {
			log.Fatalf("failed to set session status to starting: %s\n", err.Error())
		}
	}

	//endregion
}
