package PhDevHTTP

import (
	"fmt"
	//	"io/ioutil"
	"net/http"
	//	"strings"
	//	"github.com/cloudkucooland/PhDevBin"
	//  "github.com/gorilla/mux"
)

func getTagRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", "text/plain")
	fmt.Fprint(res, "tag data here.\n")
}

func deleteTagRoute(res http.ResponseWriter, req *http.Request) {
	return
}

func addUserToTagRoute(res http.ResponseWriter, req *http.Request) {
	return
}

func delUserFmTagRoute(res http.ResponseWriter, req *http.Request) {
	return
}
