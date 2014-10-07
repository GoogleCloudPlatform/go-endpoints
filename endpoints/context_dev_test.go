package endpoints

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"appengine"
	"appengine/aetest"
)

const (
	tokeinfoUserId = "12345"
	tokeinfoEmail  = "dude@gmail.com"
)

var (
	tokeninfoValid = `{
		"issued_to": "my-client-id",
		"audience": "my-client-id",
		"user_id": "` + tokeinfoUserId + `",
		"scope": "scope.one scope.two",
		"expires_in": 3600,
		"email": "` + tokeinfoEmail + `",
		"verified_email": true,
		"access_type": "online"
	}`
	tokeninfoUnverified = `{
		"expires_in": 3600,
		"verified_email": false,
		"email": "user@example.org"
	}`
	// is this even possible for email to be "" and verified == true?
	tokeninfoInvalidEmail = `{
		"expires_in": 3600,
		"verified_email": true,
		"email": ""
	}`
	tokeninfoError = `{
		"error_description": "Invalid value"
	}`
)

func TestTokeninfoContextCurrentOAuthClientID(t *testing.T) {
	rt := newTestRoundTripper()
	origTransport := httpTransportFactory
	defer func() { httpTransportFactory = origTransport }()
	httpTransportFactory = func(c appengine.Context) http.RoundTripper {
		return rt
	}

	inst, err := aetest.NewInstance(nil)
	if err != nil {
		t.Fatalf("Failed to create instance: %v", err)
	}
	defer inst.Close()

	tts := []*struct {
		token, scope, clientId string
		httpStatus             int
		content                string
	}{
		// token, scope, clientId, httpStatus, content
		{"some_token0", "scope.one", "my-client-id", 200, tokeninfoValid},
		{"some_token1", "scope.two", "my-client-id", 200, tokeninfoValid},
		{"some_token2", "scope.one", "", 200, tokeninfoUnverified},
		{"some_token3", "scope.one", "", 200, tokeninfoInvalidEmail},
		{"some_token4", "scope.one", "", 401, tokeninfoError},
		{"some_token5", "invalid.scope", "", 200, tokeninfoValid},
		{"some_token6", "scope.one", "", 400, "{}"},
		{"some_token7", "scope.one", "", 200, ""},
		{"", "scope.one", "", 200, tokeninfoValid},
		{"some_token9", "scope.one", "", -1, ""},
	}

	for i, tt := range tts {
		r, err := inst.NewRequest("GET", "/", nil)
		if err != nil {
			t.Fatalf("Error creating a req: %v", err)
		}
		r.Header.Set("authorization", "bearer "+tt.token)
		if tt.token != "" && tt.httpStatus > 0 {
			rt.Add(&http.Response{
				Status:     fmt.Sprintf("%d", tt.httpStatus),
				StatusCode: tt.httpStatus,
				Body:       ioutil.NopCloser(strings.NewReader(tt.content)),
			})
		}
		c := tokeninfoContextFactory(r)
		id, err := c.CurrentOAuthClientID(tt.scope)
		switch {
		case err != nil && tt.clientId != "":
			t.Errorf("%d: expected %q, got error %v", i, tt.clientId, err)
		case err == nil && tt.clientId == "":
			t.Errorf("%d (%v): expected error, got %q", i, tt, id)
		case err == nil && id != tt.clientId:
			t.Errorf("%d: expected %q, got %q", i, tt.clientId, id)
		}
	}
}

func TestTokeninfoCurrentOAuthUser(t *testing.T) {
	origTransport := httpTransportFactory
	defer func() {
		httpTransportFactory = origTransport
	}()
	httpTransportFactory = func(c appengine.Context) http.RoundTripper {
		return newTestRoundTripper(&http.Response{
			Status:     "200 OK",
			StatusCode: 200,
			Body:       ioutil.NopCloser(strings.NewReader(tokeninfoValid)),
		})
	}

	r, _, closer := newTestRequest(t, "GET", "/", nil)
	defer closer()
	r.Header.Set("authorization", "bearer some_token")

	c := tokeninfoContextFactory(r)
	user, err := c.CurrentOAuthUser("scope.one")

	if err != nil {
		t.Fatalf("c.CurrentOAuthUser unexpected error: %v", err)
	}
	if user.Email != tokeinfoEmail {
		t.Errorf("expected email %q, got %q", tokeinfoEmail, user.ID)
	}
}
