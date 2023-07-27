package client

import (
	"crypto/rand"
	"encoding/base64"
	"github.com/wandb/wandb/nexus/pkg/service"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"os"
	"path/filepath"
	"time"
)

func NewSettings() *service.Settings {

	rootDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	wandbDir := filepath.Join(rootDir, ".wandb")
	timeStamp := time.Now().Format("20060102_150405")
	runMode := "run"

	length := 8
	randomBytes := make([]byte, length)

	var runID string
	if _, err = rand.Read(randomBytes); err != nil {
		runID = "test"
	} else {
		runID = base64.URLEncoding.EncodeToString(randomBytes)[:length]
	}

	syncDir := filepath.Join(wandbDir, runMode+"-"+timeStamp+"-"+runID)
	logDir := filepath.Join(syncDir, "logs")

	settings := &service.Settings{
		RunId: &wrapperspb.StringValue{
			Value: runID,
		},
		BaseUrl: &wrapperspb.StringValue{
			Value: "https://api.wandb.ai",
		},
		RootDir: &wrapperspb.StringValue{
			Value: rootDir,
		},
		WandbDir: &wrapperspb.StringValue{
			Value: wandbDir,
		},
		RunMode: &wrapperspb.StringValue{
			Value: runMode,
		},
		XStartDatetime: &wrapperspb.StringValue{
			Value: timeStamp,
		},
		Timespec: &wrapperspb.StringValue{
			Value: timeStamp,
		},
		SyncDir: &wrapperspb.StringValue{
			Value: syncDir,
		},
		SyncFile: &wrapperspb.StringValue{
			Value: filepath.Join(syncDir, "run-"+runID+".wandb"),
		},
		LogDir: &wrapperspb.StringValue{
			Value: logDir,
		},
		LogInternal: &wrapperspb.StringValue{
			Value: filepath.Join(logDir, "debug-internal.log"),
		},
		LogUser: &wrapperspb.StringValue{
			Value: filepath.Join(logDir, "debug.log"),
		},
		FilesDir: &wrapperspb.StringValue{
			Value: filepath.Join(syncDir, "files"),
		},
		XDisableStats: &wrapperspb.BoolValue{
			Value: true,
		},
		XOffline: &wrapperspb.BoolValue{
			Value: true,
		},
	}
	return settings
}
