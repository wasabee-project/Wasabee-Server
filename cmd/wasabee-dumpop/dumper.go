package main

import (
	"encoding/json"
	"github.com/wasabee-project/Wasabee-Server"
)

func dumpop(gid wasabee.GoogleID, opID wasabee.OperationID) ([]byte, error) {
	var o wasabee.Operation
	o.ID = opID
	if err := o.Populate(gid); err != nil {
		wasabee.Log.Info(err)
		return nil, err
	}
	return json.MarshalIndent(o, "", "\t")
}
