package wasabee_test

import (
	"github.com/wasabee-project/Wasabee-Server"
	"os"
	"testing"
)

var gid wasabee.GoogleID

func TestMain(m *testing.M) {
	gid = wasabee.GoogleID("118281765050946915735")

	wasabee.SetupLogging(wasabee.LogConfiguration{
		Console: true,
	})
	err := wasabee.Connect(os.Getenv("DATABASE"))
	if err != nil {
		wasabee.Log.Error(err)
	}
	wasabee.SetVEnlOne(wasabee.Vconfig{
		APIKey: os.Getenv("VENLONE_API_KEY"),
	})
	wasabee.SetEnlRocks(wasabee.Rocksconfig{
		APIKey: os.Getenv("ENLROCKS_API_KEY"),
	})

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
