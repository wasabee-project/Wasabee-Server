package main

import (
	"encoding/json"
	"fmt"
	"github.com/cloudkucooland/WASABI"
	"io/ioutil"
)

func importop(gid, opfile string) error {
	g := wasabi.GoogleID(gid)

	wasabi.Log.Debugf("loading: %s", opfile)
	if opfile == "" {
		err := fmt.Errorf("no op file specified")
		wasabi.Log.Error(err)
		return err
	}

	// #nosec
	content, err := ioutil.ReadFile(opfile)
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}

	j := json.RawMessage(content)
	err = wasabi.PDrawInsert(j, g)
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}
	return nil
}
