package endpoints

import (
	"net/http"

	"appengine"
	"appengine/urlfetch"
)

// httpTransportFactory creates a new HTTP transport suitable for App Engine.
// This is made a variable on purpose, to be stubbed during testing.
var httpTransportFactory = func(c appengine.Context) http.RoundTripper {
	return &urlfetch.Transport{Context: c}
}

// newHTTPClient returns a new HTTP client using httpTransportFactory
func newHTTPClient(c appengine.Context) *http.Client {
	return &http.Client{Transport: httpTransportFactory(c)}
}
