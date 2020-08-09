package wasabee_test

import (
	"github.com/wasabee-project/Wasabee-Server"
	"os"
	"testing"
	"time"
)

var gid wasabee.GoogleID

func TestMain(m *testing.M) {
	gid = wasabee.GoogleID("118281765050946915735")

	// check these values
	wasabee.SetupDebug(false)
	wasabee.GetTimeout(2 * time.Second)
	wasabee.SetupDebug(true)
	wasabee.GetTimeout(2 * time.Second)

	wasabee.SetupLogging(wasabee.LogConfiguration{
		Console: true,
	})
	err := wasabee.Connect(os.Getenv("DATABASE"))
	if err != nil {
		wasabee.Log.Error(err)
	}

	if os.Getenv("VENLONE_API_KEY") != "" {
		wasabee.SetVEnlOne(wasabee.Vconfig{
			APIKey: os.Getenv("VENLONE_API_KEY"),
		})
	}

	/* disable rocks for now, kept messing up my communities
	if os.Getenv("ENLROCKS_API_KEY") != "" {
		wasabee.SetEnlRocks(wasabee.Rocksconfig{
			APIKey: os.Getenv("ENLROCKS_API_KEY"),
		})
	}*/

	// flag.Parse()
	exitCode := m.Run()
	wasabee.Disconnect()
	os.Exit(exitCode)
}

func TestAgentDataSetup(t *testing.T) {
	var ad wasabee.AgentData
	if err := gid.GetAgentData(&ad); err != nil {
		wasabee.Log.Error(err)
		t.Error(err.Error())
	}

	ad.Vid = wasabee.EnlID("23e27f48a04e55d6ae89188d3236d769f6629718")
	ad.Telegram.ID = 1111111111
	ad.Telegram.Verified = true

	if err := ad.Save(); err != nil {
		wasabee.Log.Error(err)
		t.Error(err.Error())
	}
}

func TestLoadWordsFile(t *testing.T) {
	err := wasabee.LoadWordsFile("testdata/small_wordlist.txt")
	if err != nil {
		t.Error(err.Error())
	}
}
