package PhDevHTTP

import (
	"fmt"
	"net/http"
    "encoding/json"
	"strings"

	"github.com/cloudkucooland/PhDevBin"
	"github.com/gorilla/mux"
)

func meShowRoute(res http.ResponseWriter, req *http.Request) {
	id, err := GetUserID(req)
	if err != nil {
		res.Header().Add("Cache-Control", "no-cache")
		PhDevBin.Log.Notice(err.Error())
		return
	}
	if id == "" {
		res.Header().Add("Cache-Control", "no-cache")
		http.Redirect(res, req, "/login", http.StatusPermanentRedirect)
		return
	}

	var ud PhDevBin.UserData
	err = PhDevBin.GetUserData(id, &ud)
	if err != nil {
		res.Header().Add("Cache-Control", "no-cache")
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	if strings.Contains(req.Referer(), "intel.ingress.com") {
        data, _ := json.Marshal(ud)
        res.Header().Add("Content-Type", "text/json")
		fmt.Fprint(res,string(data))
        return
	}

	res.Header().Add("Content-Type", "text/html")
	fmt.Fprint(res, meHeader)

	out := `
<ul>
<li>Display Name: ` + ud.IngressName + `
<form action="/me" method="get">
<input type="text" name="name" />
<input type="submit" name="update" value="update name" />
</form>
</li>
<li>Location Share Key: ` + ud.LocationKey + `</li>
<li>Tags onto which I've been invited:
  <ul>`
	for _, val := range ud.Tags {
		tmp := "<li><a href=\"/tag/" + val.Id + "\">" + val.Name + "</a> " + val.State + " <a href=\"/me/" + val.Id + "?state=On\">On</a> <a href=\"/me/" + val.Id + "?state=Off\">Off</a></li>\n"
		out = out + tmp
	}
	out = out + `
  </ul>
</li>
<li>Tags I Own:
  <ul>`
	for _, val := range ud.OwnedTags {
		tmp := "<li><a href=\"/tag/" + val.Tag + "\">" + val.Name + "</a> <a href=\"/tag/" + val.Tag + "/delete\">delete</a> <a href=\"/tag/" + val.Tag + "/edit\">edit</a></li>\n"
		out = out + tmp
	}
	out = out + `</ul>
<form action="/tag/new" method="get">
<input type="text" name="name" />
<input type="submit" name="update" value="new tag" />
</form>
</li>
<li>Draws I own:
    <ul>`
	for _, val := range ud.OwnedDraws {
		tmp := "<li>Internal ID: " + val.Hash + "<br />"
		if val.AuthTag != "" {
			tmp = tmp + "<a href=\"/tag/" + val.AuthTag + "\">Authorized Tag</a><br />"
		}
		tmp = tmp + "Uploaded: " + val.UploadTime + "<br/>Expiration: " + val.Expiration + "<br />Views: " + val.Views + "</li>\n"
		tmp = tmp + "<form action=\"/draw/" + val.Hash + "\" method=\"GET\"><input name=\"authtag\" value=\"" + val.AuthTag + "\"><input type=\"submit\" name=\"update\" value=\"Set AuthTag\"></form>\n"
		
		out = out + tmp
	}

	out = out + `</ul>
</li>
</ul>
<form action="/me" method="get">
Lat: <input type="text" name="lat" value="0" id="lat" />
Lon: <input type="text" name="lon" value="0" id="lon" />
<input type="submit" name="location" value="set location" />
</form>`

	fmt.Fprint(res, out)
	fmt.Fprint(res, meFooter)
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

func meSetUserLocationRoute(res http.ResponseWriter, req *http.Request) {
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
	lat := vars["lat"]
	lon := vars["lon"]

	// do the work
	err = PhDevBin.UserLocation(id, lat, lon)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(res, req, "/me", http.StatusPermanentRedirect)
}

const meHeader string = `<html lang="en">
<head>
<title>PhtivDraw user data</title>
    <link href="https://phtiv.com/phtivdrawtools/font-awesome/css/font-awesome.min.css" rel="stylesheet" type="text/css">
    <link href="https://fonts.googleapis.com/css?family=Lato:300,400,700,300italic,400italic,700italic" rel="stylesheet" type="text/css">
    <link href="https://phtiv.com/phtivdrawtools/css/bootstrap.min.css" rel="stylesheet">
    <link href="https://phtiv.com/phtivdrawtools/css/landing-page.css" rel="stylesheet">
</head>
<body>
    <!-- Navigation -->
    <nav class="navbar navbar-default navbar-fixed-top topnav" role="navigation">
        <div class="container topnav">
            <!-- Brand and toggle get grouped for better mobile display -->
            <div class="navbar-header">
                <button type="button" class="navbar-toggle" data-toggle="collapse" data-target="#bs-example-navbar-collapse-1">
                    <span class="sr-only">Toggle navigation</span>
                    <span class="icon-bar"></span>
                    <span class="icon-bar"></span>
                    <span class="icon-bar"></span>
                </button>
                <a class="navbar-brand topnav" href="https://phtiv.com/phtivdrawtools">PhtivDraw</a>
            </div>
            <!-- Collect the nav links, forms, and other content for toggling -->
            <div class="collapse navbar-collapse" id="bs-example-navbar-collapse-1">
                <ul class="nav navbar-nav navbar-right">
                    <li>
                        <a href="https://phtiv.com/phtivdrawtools/#contact">Contact</a>
                    </li>
                </ul>
            </div>
            <!-- /.navbar-collapse -->
        </div>
        <!-- /.container -->
    </nav>
        <div class="content-sction-a">
        <div class="container">

            <div class="row">
                <div class="col-lg-12">
                    <div class="content-section-a">
                        <ul class="list-inline">`

const meFooter string = `
                        </ul>
                    </div>
                </div>
            </div>

        </div>
        <!-- /.container -->

    </div>
    <!-- /.intro-header -->

    <footer>
        <div class="container">
            <div class="row">
                <div class="col-lg-12">
                    <ul class="list-inline">
                    </ul>
                    <p class="copyright text-muted small">Copyright &copy; Foxcutt Industries 2019. All Rights Reserved</p>
                </div>
            </div>
        </div>
    </footer>

    <script>
function geoFindMe() {
  const lat = document.querySelector('#lat');
  const lon = document.querySelector('#lon');

  function success(position) {
    lat.value = position.coords.latitude;
    lon.value = position.coords.longitude;
  }

  function error() {
    lat.value = '-0';
    lon.value = '-0';
  }

  if (navigator.geolocation) {
    navigator.geolocation.getCurrentPosition(success, error);
  }

}
document.querySelector('#lat').addEventListener('click', geoFindMe);
document.querySelector('#lon').addEventListener('click', geoFindMe);
    </script>

    <!-- jQuery -->
    <script src="https://phtiv.com/phtivdrawtools/js/jquery.js"></script>

    <!-- Bootstrap Core JavaScript -->
    <script src="https://phtiv.com/phtivdrawtools/js/bootstrap.min.js"></script>
</body>
</html>
`
