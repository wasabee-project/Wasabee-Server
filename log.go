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
