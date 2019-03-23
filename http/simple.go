package PhDevHTTP

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/cloudkucooland/PhDevBin"
	"github.com/gorilla/mux"
)

func uploadRoute(res http.ResponseWriter, req *http.Request) {
	var err error
	doc := PhDevBin.Document{}
	exp := "14d"

	// Parse form and get content
	req.Body = http.MaxBytesReader(res, req.Body, PhDevBin.MaxFilesize+1024) // MaxFilesize + 1KB metadata
	contentType := strings.Split(strings.Replace(strings.ToLower(req.Header.Get("Content-Type")), " ", "", -1), ";")[0]

	// Get the document, however the request is formatted
	if req.Method == "POST" && contentType == "application/x-www-form-urlencoded" {
		// Parse form
		err = req.ParseForm()
		if err != nil {
			PhDevBin.Log.Error(err)
			res.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(res, err.Error())
			return
		}
		doc.Content = req.PostFormValue("Q")
	} else if req.Method == "POST" && contentType == "multipart/form-data" {
		// Parse form
		err = req.ParseMultipartForm(PhDevBin.MaxFilesize + 1024)
		if err != nil {
			PhDevBin.Log.Error(err)
			res.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(res, err.Error())
			return
		}
		// Get document
		doc.Content = req.PostFormValue("Q")
		if doc.Content == "" { // Oh no, it's a file!
			// Get file
			file, _, err := req.FormFile("Q")
			if err != nil && err.Error() == "http: no such file" {
				res.WriteHeader(http.StatusBadRequest)
				fmt.Fprintf(res, "The document can't be empty.\n")
				return
			}
			if err != nil {
				PhDevBin.Log.Error(err)
				res.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(res, err.Error())
				return
			}

			// Read document
			content, err := ioutil.ReadAll(file)
			if err != nil {
				PhDevBin.Log.Error(err)
				res.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(res, err.Error())
				return
			}
			doc.Content = string(content)
		}
	} else { // PUT or POST with non-form
		// Read document
		content, err := ioutil.ReadAll(req.Body)
		if err != nil {
			PhDevBin.Log.Error(err)
			res.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(res, err.Error())
			return
		}
		doc.Content = string(content)
	}

	// Check exact filesize
	if len(doc.Content) > PhDevBin.MaxFilesize {
		res.WriteHeader(http.StatusRequestEntityTooLarge)
		fmt.Fprintf(res, "Maximum document size exceeded.\n")
		return
	}

	if len(strings.TrimSpace(doc.Content)) < 1 {
		res.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(res, "The document can't be empty (after whitespace removal).\n")
		return
	}

	/* Read metadata */
	if req.Header.Get("E") != "" {
		exp = req.Header.Get("E")
	} else if req.FormValue("E") != "" {
		exp = req.FormValue("E")
	}
	doc.Expiration, err = PhDevBin.ParseExpiration(exp)
	if err != nil {
		res.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(res, "Invalid expiration.\n")
		return
	}

	err = PhDevBin.Store(&doc)
	if err != nil && err.Error() == "file contains 0x00 bytes" {
		res.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(res, "You are trying to upload a binary file, which is not supported.\n")
		return
	} else if err != nil {
		PhDevBin.Log.Error(err)
		res.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(res, err.Error())
		return
	}

	fmt.Fprintf(res, config.Root+"/"+doc.ID+"\n")
}

func getRoute(res http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	id := vars["document"]

	doc, err := PhDevBin.Request(id)
	if err != nil {
		notFoundRoute(res, req)
	}

	res.Header().Add("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(res, "%s", doc.Content)
}
