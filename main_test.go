package wasabee_test

import (
	"firebase.google.com/go/auth"
	"fmt"
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

	// just in case of leftovers
	if err := gid.Delete(); err != nil {
		wasabee.Log.Panic(err.Error())
	}

	// start up the firebase command channel - we will consume any messages, not the firebase subsystem
	var client auth.Client
	fbchan := wasabee.FirebaseInit(&client)
	go func() {
		for fb := range fbchan {
			wasabee.Log.Infof("fbchan message: %v", fb)
		}
	}()

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

	// these values are saved
	ad.Vid = wasabee.EnlID("23e27f48a04e55d6ae89188d3236d769f6629718")
	ad.Telegram.ID = 1111111111
	ad.Telegram.Verified = true

	tgid := wasabee.TelegramID(ad.Telegram.ID)

	wasabee.Log.Infof("%v", ad)

	if err := ad.Save(); err != nil {
		wasabee.Log.Error(err)
		t.Error(err.Error())
	}
	if _, err := gid.NewLocKey(); err != nil {
		wasabee.Log.Error(err)
		t.Error(err.Error())
	}

	fgid, err := wasabee.SearchAgentName("unknown")
	if err != nil {
		t.Errorf(err.Error())
	}
	wasabee.Log.Infof("got '%s' for 'unknown'", fgid)

	fgid, err = wasabee.SearchAgentName("@unused")
	if err != nil {
		t.Errorf(err.Error())
	}
	wasabee.Log.Infof("got '%s' for '@unused'", fgid)

	fgid, err = wasabee.SearchAgentName("@deviousness")
	if err != nil {
		t.Errorf(err.Error())
	}
	wasabee.Log.Infof("got '%s' for '@deviousness'", fgid)

	if err = tgid.UpdateName("deviousness"); err != nil {
		t.Errorf(err.Error())
	}
	fgid, err = wasabee.SearchAgentName("@deviousness")
	if err != nil {
		t.Errorf(err.Error())
	}
	wasabee.Log.Infof("got '%s' for '@deviousness'", fgid)
}

func TestLoadWordsFile(t *testing.T) {
	err := wasabee.LoadWordsFile("/dev/null")
	if err != nil {
		wasabee.Log.Info("correctly reported empty word list")
	} else {
		err := fmt.Errorf("did not report empty word list")
		t.Error(err.Error())
	}

	err = wasabee.LoadWordsFile("/tmp/_no_such_file")
	if err != nil {
		wasabee.Log.Info("correctly reported missing word file")
	} else {
		err := fmt.Errorf("did not report missing word file")
		t.Error(err.Error())
	}

	err = wasabee.LoadWordsFile("testdata/small_wordlist.txt")
	if err != nil {
		t.Error(err.Error())
	}
}
