package PhDevHTTP

import (
	"fmt"
	"net/http"

	"github.com/cloudkucooland/PhDevBin"
	"github.com/gorilla/mux"
)

func meShowRoute(res http.ResponseWriter, req *http.Request) {
	id, err := GetUserID(req)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		return
	}
	if id == "" {
		http.Redirect(res, req, "/login", http.StatusPermanentRedirect)
		return
	}

	var ud PhDevBin.UserData
	err = PhDevBin.GetUserData(id, &ud)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	res.Header().Add("Content-Type", "text/html")
	out :=
		`<html>
<head>
<title>PhtivDraw user data</title>
</head>
<body>
<ul>
<li>Display Name: ` + ud.IngressName +
			`</li>
<li>Location Share Key: ` + ud.LocationKey +
			`</li>
<li>Member Tags:
  <ul>`
	for _, val := range ud.Tags {
		tmp := "<li><a href=\"/tag/" + val.Id + "\">" +  val.Name + "</a> " + val.State + " <a href=\"/me/" + val.Id + "?state=On\">On</a> <a href=\"/me/" + val.Id + "?state=Off\">Off</a></li>\n"
		out = out + tmp
	}
	out = out +
		`
  </ul>
</li>
<li>Owned Tags:
  <ul>`
	for _, val := range ud.OwnedTags {
		tmp := "<li><a href=\"/tag/" + val.Tag + "\">" +  val.Name + "</a> <a href=\"/tag/" + val.Tag + "/delete\">delete</a></li>\n"
		out = out + tmp
	}
	out = out +
`
  </ul>
</li>
</ul>
<form action="/me" method="get">
<input type="text" name="name" />
<input type="submit" name="update" value="update name" />
</form>
<form action="/tag/new" method="get">
<input type="text" name="name" />
<input type="submit" name="update" value="new tag" />
</form>
</body>
</html>
`
	fmt.Fprint(res, out)
}

func meToggleTagRoute(res http.ResponseWriter, req *http.Request) {
	id, err := GetUserID(req)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if id == "" {
		http.Redirect(res, req, "/login", http.StatusPermanentRedirect)
		return
	}

	vars := mux.Vars(req)
	tag := vars["tag"]
	state := vars["state"]

	err = PhDevBin.SetUserTagState(id, tag, state)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(res, req, "/me", http.StatusPermanentRedirect)
}

func meRemoveTagRoute(res http.ResponseWriter, req *http.Request) {
	id, err := GetUserID(req)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if id == "" {
		http.Redirect(res, req, "/login", http.StatusPermanentRedirect)
		return
	}

	vars := mux.Vars(req)
	tag := vars["tag"]

	// do the work
	PhDevBin.Log.Notice("remove me from tag: " + id + " " + tag)
	err = PhDevBin.RemoveUserFromTag(id, tag)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(res, req, "/me", http.StatusPermanentRedirect)
}

func meSetIngressNameRoute(res http.ResponseWriter, req *http.Request) {
	id, err := GetUserID(req)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if id == "" {
		http.Redirect(res, req, "/login", http.StatusPermanentRedirect)
		return
	}

	vars := mux.Vars(req)
	name := vars["name"]

	// do the work
	err = PhDevBin.SetIngressName(id, name)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(res, req, "/me", http.StatusPermanentRedirect)
}
