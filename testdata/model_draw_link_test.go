package wasabee_test

import (
	"encoding/json"
	// "fmt"
	"github.com/wasabee-project/Wasabee-Server"
	"io"
	"testing"
)

func TestDistance(t *testing.T) {
	content, err := io.ReadFile("testdata/test1.json")
	if err != nil {
		t.Error(err.Error())
	}

	j := json.RawMessage(content)
	var in wasabee.Operation
	err = json.Unmarshal(j, &in)
	if err != nil {
		t.Error(err.Error())
	}

	p := make(map[wasabee.PortalID]wasabee.Portal)

	for _, portal := range in.OpPortals {
		p[portal.ID] = portal
	}

	for _, link := range in.Links {
		d := wasabee.Distance(p[link.From].Lat, p[link.From].Lon, p[link.To].Lat, p[link.To].Lon)
		m := wasabee.MinPortalLevel(d, 8, true)
		wasabee.Log.Infof("distance: %.3f km = min level: %.1f", (d / 1000), m)
	}
}
