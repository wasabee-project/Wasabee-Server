package wasabihttps

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cloudkucooland/WASABI"
	"github.com/gorilla/mux"
)

func uploadRoute(res http.ResponseWriter, req *http.Request) {
	var err error
	doc := wasabi.SimpleDocument{}
	exp := "14d"

	// Parse form and get content
	req.Body = http.MaxBytesReader(res, req.Body, wasabi.MaxFilesize+1024) // MaxFilesize + 1KB metadata
	contentType := strings.Split(strings.Replace(strings.ToLower(req.Header.Get("Content-Type")), " ", "", -1), ";")[0]

	// Get the document, however the request is formatted
	if req.Method == "POST" && contentType == "application/x-www-form-urlencoded" {
		// Parse form
		err = req.ParseForm()
		if err != nil {
			wasabi.Log.Error(err)
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}
		doc.Content = req.PostFormValue("Q")
	} else if req.Method == "POST" && contentType == "multipart/form-data" {
		// Parse form
		err = req.ParseMultipartForm(wasabi.MaxFilesize + 1024)
		if err != nil {
			wasabi.Log.Error(err)
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}
		// Get document
		doc.Content = req.PostFormValue("Q")
		if doc.Content == "" { // Oh no, it's a file!
			// Get file
			file, _, err := req.FormFile("Q")
			if err != nil && err.Error() == "http: no such file" {
				err = fmt.Errorf("the document can't be empty")
				wasabi.Log.Error(err)
				http.Error(res, err.Error(), http.StatusBadRequest)
				return
			}
			if err != nil {
				wasabi.Log.Error(err)
				http.Error(res, err.Error(), http.StatusBadRequest)
				return
			}

			// Read document
			content, err := ioutil.ReadAll(file)
			if err != nil {
				wasabi.Log.Error(err)
				http.Error(res, err.Error(), http.StatusBadRequest)
				return
			}
			doc.Content = string(content)
		}
	} else { // PUT or POST with non-form
		// Read document
		content, err := ioutil.ReadAll(req.Body)
		if err != nil {
			wasabi.Log.Error(err)
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}
		doc.Content = string(content)
	}

	// Check exact filesize
	if len(doc.Content) > wasabi.MaxFilesize {
		err = fmt.Errorf("maximum document size exceeded")
		wasabi.Log.Error(err)
		http.Error(res, err.Error(), http.StatusRequestEntityTooLarge)
		return
	}

	if len(strings.TrimSpace(doc.Content)) < 1 {
		err = fmt.Errorf("the document can't be empty (after whitespace removal)")
		wasabi.Log.Error(err)
		http.Error(res, err.Error(), http.StatusBadRequest)
		return
	}

	// Read settings
	if req.Header.Get("E") != "" {
		exp = req.Header.Get("E")
	} else if req.FormValue("E") != "" {
		exp = req.FormValue("E")
	}
	doc.Expiration, err = parseExpiration(exp)
	if err != nil {
		err = fmt.Errorf("invalid expiration")
		wasabi.Log.Error(err)
		http.Error(res, err.Error(), http.StatusBadRequest)
		return
	}

	dp := &doc
	err = dp.Store()
	if err != nil && err.Error() == "file contains 0x00 bytes" {
		err = fmt.Errorf("binary file upload is not supported")
		wasabi.Log.Error(err)
		http.Error(res, err.Error(), http.StatusBadRequest)
		return
	} else if err != nil {
		wasabi.Log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(res, "%s/simple/%s\n", config.Root, doc.ID)
}

func getRoute(res http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	id := vars["document"]

	doc, err := wasabi.Request(id)
	if err != nil {
		notFoundRoute(res, req)
	}

	res.Header().Add("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprint(res, doc.Content)
}

// ParseExpiration creates a time.Time object from an expiration string, taking the units m, h, d, w into account.
func parseExpiration(expiration string) (time.Time, error) {
	expiration = strings.ToLower(strings.TrimSpace(expiration))
	if expiration == "volatile" {
		return time.Unix(-1, 0), nil
	}

	var multiplier int64

	if strings.HasSuffix(expiration, "h") {
		expiration = strings.TrimSuffix(expiration, "h")
		multiplier = 60
	} else if strings.HasSuffix(expiration, "d") {
		expiration = strings.TrimSuffix(expiration, "d")
		multiplier = 60 * 24
	} else if strings.HasSuffix(expiration, "w") {
		expiration = strings.TrimSuffix(expiration, "w")
		multiplier = 60 * 24 * 7
	} else {
		expiration = strings.TrimSuffix(expiration, "m")
		multiplier = 1
	}

	value, err := strconv.ParseInt(expiration, 10, 0)
	if err != nil {
		return time.Time{}, err
	}

	if multiplier*value == 0 {
		return time.Time{}, nil
	}

	expirationTime := time.Now().Add(time.Duration(multiplier*value) * time.Minute)

	return expirationTime, nil
}
