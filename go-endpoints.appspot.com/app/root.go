package app

import (
	"html/template"
	"net/http"
	"path/filepath"

	"appengine"
)

const (
	apiExplorerUrl = "https://developers.google.com/apis-explorer/"
)

func handler(w http.ResponseWriter, r *http.Request) {
	t, err := template.ParseFiles(filepath.Join("templates", "home.html"))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	data := &struct {
		BaseUrl        string
		ApiExplorerUrl string
	}{
		BaseUrl:        getScheme(r) + "://" + r.Host,
		ApiExplorerUrl: apiExplorerUrl,
	}
	if err := t.Execute(w, data); err != nil {
		http.Error(w, err.Error(), 500)
	}
}

func getScheme(r *http.Request) string {
	scheme := r.URL.Scheme
	if scheme == "" {
		if appengine.IsDevAppServer() {
			scheme = "http"
		} else {
			scheme = "https"
		}
	}
	return scheme
}

func init() {
	http.HandleFunc("/", handler)
}
