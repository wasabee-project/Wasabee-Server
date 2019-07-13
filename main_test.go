package wasabee_test

import (
	"github.com/wasabee-project/Wasabee-Server"
	"github.com/op/go-logging"
	"os"
	"testing"
)

var gid wasabee.GoogleID

func TestMain(m *testing.M) {
	gid = wasabee.GoogleID("118281765050946915735")

	wasabee.SetLogLevel(logging.DEBUG)
	err := wasabee.Connect(os.Getenv("DATABASE"))
	if err != nil {
		wasabee.Log.Error(err)
	}
	wasabee.SetVEnlOne(os.Getenv("VENLONE_API_KEY"))
	wasabee.SetEnlRocks(os.Getenv("ENLROCKS_API_KEY"))

	// flag.Parse()
	exitCode := m.Run()
	wasabee.Disconnect()
	os.Exit(exitCode)
}

func TestLoadWordsFile(t *testing.T) {
	err := wasabee.LoadWordsFile("testdata/small_wordlist.txt")
	if err != nil {
		t.Error(err.Error())
	}
}
