package wasabi_test

import (
	"fmt"
	"github.com/cloudkucooland/WASABI"
	"strconv"
	"testing"
)

func TestVConfigured(t *testing.T) {
	b := wasabi.GetvEnlOne()
	if b != true {
		t.Errorf("V API Key not configured")
	}
}

func TestVsearch(t *testing.T) {
	var v wasabi.Vresult

	err := gid.VSearch(&v)
	if err != nil {
		t.Errorf(err.Error())
	}
	fmt.Printf("%s: %s\n", v.Status, v.Message)
	if v.Status != "ok" {
		t.Errorf("V Status: %s", v.Status)
	}

	if v.Data.Agent != "deviousness" {
		t.Errorf("V agent found: %s", v.Status)
	}
}

func TestStatusLocation(t *testing.T) {
	lat, lon, err := gid.StatusLocation()
	if err != nil {
		t.Errorf(err.Error())
	}
	var fLat, fLon float64
	fLat, _ = strconv.ParseFloat(lat, 64)
	fLon, _ = strconv.ParseFloat(lon, 64)
	if fLat > 90.0 || fLat < -90.0 {
		t.Errorf("impossible lat: %f", fLat)
	}
	if fLon > 180.0 || fLon < -180.0 {
		t.Errorf("impossible lon: %f", fLon)
	}
}

func TestGid(t *testing.T) {
	eid := wasabi.EnlID("23e27f48a04e55d6ae89188d3236d769f6629718")
	fgid, err := eid.Gid()
	if err != nil {
		t.Errorf(err.Error())
	}
	if gid.String() != fgid.String() {
		t.Errorf("EnlID(%s) = Gid(%s); expecting Gid(%s)", eid.String(), fgid.String(), fgid.String())
	}
}
