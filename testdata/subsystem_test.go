package wasabee_test

import (
	"github.com/wasabee-project/Wasabee-Server"
	"testing"
)

// nothing to do here, just run the code
func TestPubsubCmdChan(t *testing.T) {
	go wasabee.PubSubInit()

	// wasabee.PubSubClose()
}

func TestFirebaseCmdChan(t *testing.T) {
	// go wasabee.FirebaseInit()

	gid.FirebaseGenericMessage("fake mesage")

	err := gid.FirebaseInsertToken("fake token")
	if err != nil {
		t.Error(err.Error())
	}

	// double insert should be ignored w/ warning
	err = gid.FirebaseInsertToken("fake token")
	if err != nil {
		t.Error(err.Error())
	}

	// second token
	err = gid.FirebaseInsertToken("fake token 2")
	if err != nil {
		t.Error(err.Error())
	}

	toks, err := gid.FirebaseTokens()
	if err != nil {
		t.Error(err.Error())
	}
	for _, tok := range toks {
		wasabee.Log.Info(tok)
		gid.FirebaseRemoveToken(tok)
		if err != nil {
			t.Error(err.Error())
		}
	}

	err = gid.FirebaseInsertToken("fake token")
	if err != nil {
		t.Error(err.Error())
	}

	gid.FirebaseRemoveAllTokens()
	if err != nil {
		t.Error(err.Error())
	}
}
