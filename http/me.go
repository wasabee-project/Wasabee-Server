package PhDevHTTP

import (
	"fmt"
//	"io/ioutil"
	"net/http"
//	"strings"

	"github.com/cloudkucooland/PhDevBin"
//  "github.com/gorilla/mux"
)

func meShowRoute(res http.ResponseWriter, req *http.Request) {
    id, err := GetUserID(req)
	if err != nil {
        PhDevBin.Log.Notice(err.Error())
		return
	}

    res.Header().Add("Content-Type", "text/plain")
	fmt.Fprint(res, "a screen full of data about me will be here.\n")
	fmt.Fprint(res, "user ID: " + id + "\n")
	fmt.Fprint(res, "google name: \n")
	fmt.Fprint(res, "ingress handle: \n")
	fmt.Fprint(res, "location share key: \n")
}

func meToggleTagRoute(res http.ResponseWriter, req *http.Request) {
    return
}

func meRemoveTagRoute(res http.ResponseWriter, req *http.Request) {
    return
}

func meSetColorRoute(res http.ResponseWriter, req *http.Request) {
    return
}

