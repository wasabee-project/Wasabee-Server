package WASABI_test

import (
	"fmt"
	"testing"
	"os"
	"github.com/cloudkucooland/WASABI"
)

func TestMain(m *testing.M) {
	err := WASABI.Connect(os.Getenv("DATABASE"))
	if err != nil {
		WASABI.Log.Error(err)
	}
	WASABI.SetVEnlOne(os.Getenv("VENLONE_API_KEY"))

	// flag.Parse()
	exitCode := m.Run()
	os.Exit(exitCode)
}

func TestEasy(t *testing.T) {
	b := WASABI.GetvEnlOne()
	if b != true {
		t.Errorf("V API Key not configured")
	}
}

func TestVsearch(t *testing.T) {
	var v WASABI.Vresult
	gid := WASABI.GoogleID("118281765050946915735")

	err := gid.VSearch(&v)
	if err != nil {
		t.Errorf(err.Error())
	}
	fmt.Printf("%s: %s\n",v.Status, v.Message)
	if v.Status != "ok" {
		t.Errorf("V Status: %s", v.Status)
	}

	if v.Data.Agent != "deviousness" {
		t.Errorf("V agent found: %s", v.Status)
	}
}
