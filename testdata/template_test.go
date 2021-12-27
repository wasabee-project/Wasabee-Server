package wasabee_test

import (
	"github.com/wasabee-project/Wasabee-Server"
	"testing"
)

func TestTemplate(t *testing.T) {
	// this segfaults if the testdata/templates/master and testdata/templates/en do not exist
	// it should be more robust and err out safely

	_, err := wasabee.TemplateConfig("testdata/templates")
	if err != nil {
		t.Error(err.Error())
	}

	out, err := gid.ExecuteTemplate("test", "env")
	if err != nil {
		t.Error(err.Error())
	}
	wasabee.Log.Info(out)
	out, err = gid.ExecuteTemplate("One", "")
	if err != nil {
		t.Error(err.Error())
	}
	wasabee.Log.Info(out)
	out, err = gid.ExecuteTemplate("Two", gid)
	if err != nil {
		t.Error(err.Error())
	}
	wasabee.Log.Info(out)
}
