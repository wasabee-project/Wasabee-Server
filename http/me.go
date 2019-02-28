package PhDevHTTP

import (
	"encoding/json"
	"fmt"
	"net/http"
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
		fmt.Fprint(res, string(data))
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
<li>Teams onto which I've been invited:
  <ul>`
	for _, val := range ud.Teams {
		tmp := "<li><a href=\"/team/" + val.Id + "\">" + val.Name + "</a> " + val.State + " <a href=\"/me/" + val.Id + "?state=On\">On</a> <a href=\"/me/" + val.Id + "?state=Off\">Off</a></li>\n"
		out = out + tmp
	}
	out = out + `
  </ul>
</li>
<li>Teams I Own:
  <ul>`
	for _, val := range ud.OwnedTeams {
		tmp := "<li><a href=\"/team/" + val.Team + "\">" + val.Name + "</a> <a href=\"/team/" + val.Team + "/delete\">delete</a> <a href=\"/team/" + val.Team + "/edit\">edit</a></li>\n"
		out = out + tmp
	}
	out = out + `</ul>
<form action="/team/new" method="get">
<input type="text" name="name" />
<input type="submit" name="update" value="new team" />
</form>
</li>
<li>Draws I own:
	<ul>`
	for _, val := range ud.OwnedDraws {
		tmp := "<li>Internal ID: " + val.Hash + "<br />"
		if val.AuthTeam != "" {
			tmp = tmp + "<a href=\"/team/" + val.AuthTeam + "\">Authorized Team</a><br />"
		}
		tmp = tmp + "Uploaded: " + val.UploadTime + "<br/>Expiration: " + val.Expiration + "<br />Views: " + val.Views + "</li>\n"
		tmp = tmp + "<form action=\"/draw/" + val.Hash + "\" method=\"GET\"><input name=\"authteam\" value=\"" + val.AuthTeam + "\"><input type=\"submit\" name=\"update\" value=\"Set AuthTeam\"></form>\n"

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

func meToggleTeamRoute(res http.ResponseWriter, req *http.Request) {
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
	team := vars["team"]
	state := vars["state"]

	err = PhDevBin.SetUserTeamState(id, team, state)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(res, req, "/me", http.StatusPermanentRedirect)
}

func meRemoveTeamRoute(res http.ResponseWriter, req *http.Request) {
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
	team := vars["team"]

	// do the work
	PhDevBin.Log.Notice("remove me from team: " + id + " " + team)
	err = PhDevBin.RemoveUserFromTeam(id, team)
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
