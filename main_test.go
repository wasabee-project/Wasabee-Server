package wasabi_test

import (
	"github.com/cloudkucooland/WASABI"
	"github.com/op/go-logging"
	"os"
	"testing"
)

var gid wasabi.GoogleID

func TestMain(m *testing.M) {
	gid = wasabi.GoogleID("118281765050946915735")

	wasabi.SetLogLevel(logging.DEBUG)
	err := wasabi.Connect(os.Getenv("DATABASE"))
	if err != nil {
		wasabi.Log.Error(err)
	}
	wasabi.SetVEnlOne(os.Getenv("VENLONE_API_KEY"))
	wasabi.SetEnlRocks(os.Getenv("ENLROCKS_API_KEY"))

	// flag.Parse()
	exitCode := m.Run()
	wasabi.Disconnect()
	os.Exit(exitCode)
}

func TestLoadWordsFile(t *testing.T) {
	err := wasabi.LoadWordsFile("testdata/small_wordlist.txt")
	if err != nil {
		t.Error(err.Error())
	}
}
