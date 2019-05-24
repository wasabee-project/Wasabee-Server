package wasabi

import (
	"os"

	"github.com/op/go-logging"
)

// Log refers to the main logger instance used by the WASABI application.
var Log = logging.MustGetLogger("WASABI")
var leveled logging.LeveledBackend

// SetLogLevel changes how much information is printed to Stdout.
func SetLogLevel(level logging.Level) {
	leveled.SetLevel(level, "")
}

func init() {
	backend := logging.NewLogBackend(os.Stdout, "", 0)
	format := logging.MustStringFormatter(`%{color}%{time:15:04:05.000} %{shortfunc}: %{level:.4s} %{color:reset} %{message}`)
	formatter := logging.NewBackendFormatter(backend, format)
	leveled = logging.AddModuleLevel(formatter)
	leveled.SetLevel(logging.NOTICE, "")

	logging.SetBackend(leveled)
}

// AddFileLog duplicates the console log to a file
func AddFileLog(lf string, level logging.Level) error {
	// #nosec
	logfile, err := os.OpenFile(lf, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		Log.Critical("unable to open log file")
		return err
	}

	backend := logging.NewLogBackend(logfile, "", 0)
	format := logging.MustStringFormatter(`%{time:15:04:05.000} %{shortfunc}: %{level:.4s} %{message}`)
	formatter := logging.NewBackendFormatter(backend, format)
	leveled2 := logging.AddModuleLevel(formatter)
	leveled2.SetLevel(level, "")

	multi := logging.MultiLogger(leveled, leveled2)
	logging.SetBackend(multi)
	return nil
}
