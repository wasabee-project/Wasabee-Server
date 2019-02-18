package PhDevHTTP

import (
	"encoding/json"
	"fmt"
	"github.com/cloudkucooland/PhDevBin"
	"github.com/gorilla/mux"
	"net/http"
)

func getTagRoute(res http.ResponseWriter, req *http.Request) {
	var tagList []PhDevBin.TagData

	id, err := GetUserID(req)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if id == "" {
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(req)
	tag := vars["tag"]

	safe, err := PhDevBin.UserInTag(id, tag, false)
	if safe {
		PhDevBin.FetchTag(tag, &tagList, false)
		data, _ := json.Marshal(tagList)
		s := string(data)
		res.Header().Add("Content-Type", "text/json")
		fmt.Fprintf(res, s)
		return
	}
	http.Error(res, "Unauthorized", http.StatusUnauthorized)
}

func newTagRoute(res http.ResponseWriter, req *http.Request) {
	id, err := GetUserID(req)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if id == "" {
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(req)
	name := vars["name"]
	_, err = PhDevBin.NewTag(name, id)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(res, req, "/me", http.StatusPermanentRedirect)
}

func deleteTagRoute(res http.ResponseWriter, req *http.Request) {
	id, err := GetUserID(req)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if id == "" {
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(req)
	tag := vars["tag"]
	safe, err := PhDevBin.UserOwnsTag(id, tag)
	if safe != true {
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}
	err = PhDevBin.DeleteTag(tag)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(res, req, "/me", http.StatusPermanentRedirect)
}

func editTagRoute(res http.ResponseWriter, req *http.Request) {
	id, err := GetUserID(req)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if id == "" {
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(req)
	tag := vars["tag"]
	safe, err := PhDevBin.UserOwnsTag(id, tag)
	if safe != true {
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}
	var tagList []PhDevBin.TagData
	err = PhDevBin.FetchTag(tag, &tagList, true)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	res.Header().Add("Content-Type", "text/html")
	out :=
		`<html>
<head>
<title>PhtivDraw edit tag: ` + tag + `</title>
</head>
<body>
<ul>`
	for _, val := range tagList {
		tmp := "<li>" + val.LocKey + ": " + val.Name + " (" + val.State + ") <a href=\"/tag/" + tag + "/" + val.LocKey + "/delete\">remove</a></li>\n"
		out = out + tmp
	}
	out = out +
		`
</ul>
</body>
<form action="/tag/` + tag + `" method="get">
Location Key: <input type="text" name="key" />
<input type="submit" name="add" value="add user to tag" />
</form>
</html>`
	fmt.Fprint(res, out)
}

func addUserToTagRoute(res http.ResponseWriter, req *http.Request) {
	id, err := GetUserID(req)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if id == "" {
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(req)
	tag := vars["tag"]
	key := vars["key"]

	safe, err := PhDevBin.UserOwnsTag(id, tag)
	if safe != true {
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}
	err = PhDevBin.AddUserToTag(tag, key)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(res, req, "/tag/"+tag+"/edit", http.StatusPermanentRedirect)
}

func delUserFmTagRoute(res http.ResponseWriter, req *http.Request) {
	id, err := GetUserID(req)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if id == "" {
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(req)
	tag := vars["tag"]
	key := vars["key"]
	safe, err := PhDevBin.UserOwnsTag(id, tag)
	if safe != true {
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}
	err = PhDevBin.DelUserFromTag(tag, key)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(res, req, "/tag/"+tag+"/edit", http.StatusPermanentRedirect)
}
