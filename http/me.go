package PhDevHTTP

import (
	"fmt"
//	"io/ioutil"
	"net/http"
//	"strings"

//	"github.com/cloudkucooland/PhDevBin"
//  "github.com/gorilla/mux"
)

func meShowRoute(res http.ResponseWriter, req *http.Request) {
    res.Header().Add("Content-Type", "text/plain")
	fmt.Fprint(res, "a screen full of data about me will be here.\n")
}

func meToggleTagRoute(res http.ResponseWriter, req *http.Request) {
    return
}

func meRemoveTagRoute(res http.ResponseWriter, req *http.Request) {
    return

func meSetColorRoute(res http.ResponseWriter, req *http.Request) {
    return
}

