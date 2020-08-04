package wasabee

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"

	"cloud.google.com/go/logging"
	"github.com/jonstaryuk/gcloudzap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/api/option"
	// "github.com/blendle/zapdriver"
)

var Log *zap.SugaredLogger

type LogConfiguration struct {
	Console            bool
	ConsoleLevel       zapcore.Level
	GoogleCloudProject string
	GoogleCloudCreds   string
	FilePath           string
	FileLevel          zapcore.Level
}

func SetupLogging(c LogConfiguration) {
	var cores []zapcore.Core

	if c.Console {
		atom := zap.NewAtomicLevel()
		atom.SetLevel(c.ConsoleLevel)
		encoderCfg := zap.NewDevelopmentEncoderConfig()
		encoderCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
		consoleCore := zapcore.NewCore(
			zapcore.NewConsoleEncoder(encoderCfg),
			zapcore.Lock(os.Stdout),
			atom,
		)
		cores = append(cores, consoleCore)
	}

	if c.FilePath != "" {
		fileCore, err := addFileLog(c.FilePath, c.FileLevel)
		if err != nil {
			fmt.Printf("Unable to open log file, %s: %v\n", c.FilePath, err)
		} else {
			cores = append(cores, fileCore)
		}
	}

	if c.GoogleCloudProject != "" && c.GoogleCloudCreds != "" {
		gcCore, err := addGoogleCloud(c.GoogleCloudProject, c.GoogleCloudCreds)
		if err != nil {
			fmt.Printf("unable to start cloud logging to project %s with creds %s: %v\n", c.GoogleCloudProject, c.GoogleCloudCreds, err)
		} else {
			cores = append(cores, gcCore)
		}
	}

	tee := zapcore.NewTee(cores...)
	sugarfree := zap.New(tee)
	undo, err := zap.RedirectStdLogAt(sugarfree, zap.DebugLevel)
	if err != nil {
		undo()
	}

	Log = sugarfree.Sugar()
	Log.Sync()
}

func addGoogleCloud(project string, jsonPath string) (zapcore.Core, error) {
	ctx := context.Background()
	opt := option.WithCredentialsFile(jsonPath)

	atom := zap.NewAtomicLevel()
	atom.SetLevel(zap.InfoLevel)
	encoderCfg := zap.NewProductionEncoderConfig()
	inCore := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderCfg),
		zapcore.AddSync(ioutil.Discard),
		atom,
	)

	client, err := logging.NewClient(ctx, project, opt)
	if err != nil {
		return nil, err
	}

	hn, err := os.Hostname()
	if err != nil {
		hn = "wasabee-server"
	}
	gcore := gcloudzap.Tee(inCore, client, hn)
	return gcore, nil
}

// AddFileLog duplicates the console log to a file
func addFileLog(logfile string, level zapcore.Level) (zapcore.Core, error) {
	// #nosec
	lf, err := os.OpenFile(logfile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return nil, err
	}

	atom := zap.NewAtomicLevel()
	atom.SetLevel(level)

	encoderCfg := zap.NewDevelopmentEncoderConfig()
	fileCore := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderCfg),
		zapcore.Lock(lf),
		atom,
	)

	return fileCore, nil
}
