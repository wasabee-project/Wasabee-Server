package main

import (
	"encoding/json"
	"fmt"
	"github.com/wasabee-project/Wasabee-Server"
	"io/ioutil"
)

func importop(gid, opfile string) error {
	g := wasabee.GoogleID(gid)

	wasabee.Log.Debugf("loading: %s", opfile)
	if opfile == "" {
		err := fmt.Errorf("no op file specified")
		wasabee.Log.Error(err)
		return err
	}

	// #nosec
	content, err := ioutil.ReadFile(opfile)
	if err != nil {
		wasabee.Log.Error(err)
		return err
	}

	j := json.RawMessage(content)
	err = wasabee.DrawInsert(j, g)
	if err != nil {
		wasabee.Log.Error(err)
		return err
	}
	return nil
}
