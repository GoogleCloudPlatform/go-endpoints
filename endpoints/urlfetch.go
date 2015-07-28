package endpoints

import (
	"net/http"

	"golang.org/x/net/context"
	"google.golang.org/appengine/urlfetch"
)

// httpTransportFactory creates a new HTTP transport suitable for App Engine.
// This is made a variable on purpose, to be stubbed during testing.
var httpTransportFactory = func(c context.Context) http.RoundTripper {
	return &urlfetch.Transport{Context: c}
}

// newHTTPClient returns a new HTTP client using httpTransportFactory
func newHTTPClient(c context.Context) *http.Client {
	return &http.Client{Transport: httpTransportFactory(c)}
}
