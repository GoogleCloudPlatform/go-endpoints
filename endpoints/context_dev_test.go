package endpoints

import (
	"errors"
	"testing"

	fetch_pb "appengine_internal/urlfetch"
	"code.google.com/p/goprotobuf/proto"

	tu "github.com/crhym3/aegot/testutils"
)

const (
	tokeinfoUserId = "12345"
	tokeinfoEmail  = "dude@gmail.com"
)

var (
	tokeninfoValid = []byte(`{
		"issued_to": "my-client-id",
		"audience": "my-client-id",
		"user_id": "` + tokeinfoUserId + `",
		"scope": "scope.one scope.two",
		"expires_in": 3600,
		"email": "` + tokeinfoEmail + `",
		"verified_email": true,
		"access_type": "online"
	}`)
	tokeninfoUnverified = []byte(`{
		"expires_in": 3600,
		"verified_email": false,
		"email": "user@example.org"
	}`)
	// is this even possible for email to be "" and verified == true?
	tokeninfoInvalidEmail = []byte(`{
		"expires_in": 3600,
		"verified_email": true,
		"email": ""
	}`)
	tokeninfoError = []byte(`{
		"error_description": "Invalid value"
	}`)
)

func TestTokeninfoContextCurrentOAuthClientID(t *testing.T) {
	const token = "some_token"

	type test struct {
		token, scope, clientId string
		httpStatus             int32
		content                []byte
		fetchErr               error
	}

	var currTT *test

	fetchStub := func(in, out proto.Message, _ *tu.RpcCallOptions) error {
		req := in.(*fetch_pb.URLFetchRequest)
		url := tokeninfoEndpointUrl + "?access_token=" + token
		if req.GetUrl() != url {
			t.Errorf("fetch: expected URL %q, got %q", url, req.GetUrl())
		}
		resp := out.(*fetch_pb.URLFetchResponse)
		resp.StatusCode = proto.Int32(currTT.httpStatus)
		resp.Content = currTT.content
		return currTT.fetchErr
	}
	defer tu.RegisterAPIOverride("urlfetch", "Fetch", fetchStub)()

	r, deleteCtx := tu.NewTestRequest("GET", "/", nil)
	defer deleteCtx()

	tts := []*test{
		// token, scope, clientId, httpStatus, content, fetchErr
		{token, "scope.one", "my-client-id", 200, tokeninfoValid, nil},
		{token, "scope.two", "my-client-id", 200, tokeninfoValid, nil},
		{token, "scope.one", "", 200, tokeninfoUnverified, nil},
		{token, "scope.one", "", 200, tokeninfoInvalidEmail, nil},
		{token, "scope.one", "", 401, tokeninfoError, nil},
		{token, "invalid.scope", "", 200, tokeninfoValid, nil},
		{token, "scope.one", "", 400, []byte("{}"), nil},
		{token, "scope.one", "", 200, []byte(""), nil},
		{token, "scope.one", "", -1, nil, errors.New("Fake urlfetch error")},
		{"", "scope.one", "", 200, tokeninfoValid, nil},
	}

	c := tokeninfoContextFactory(r)
	for i, tt := range tts {
		currTT = tt
		r.Header.Set("authorization", "bearer "+tt.token)
		id, err := c.CurrentOAuthClientID(tt.scope)
		switch {
		case err != nil && tt.clientId != "":
			t.Errorf("%d: expected %q, got error %v", i, tt.clientId, err)
		case err == nil && tt.clientId == "":
			t.Errorf("%d: expected error, got %q", i, id)
		case err == nil && id != tt.clientId:
			t.Errorf("%d: expected %q, got %q", i, tt.clientId, id)
		}
	}
}

func TestTokeninfoCurrentOAuthUser(t *testing.T) {
	fetchStub := func(in, out proto.Message, _ *tu.RpcCallOptions) error {
		resp := out.(*fetch_pb.URLFetchResponse)
		resp.StatusCode = proto.Int32(200)
		resp.Content = tokeninfoValid
		return nil
	}
	defer tu.RegisterAPIOverride("urlfetch", "Fetch", fetchStub)()

	r, deleteCtx := tu.NewTestRequest("GET", "/", nil)
	defer deleteCtx()
	r.Header.Set("authorization", "bearer some_token")

	c := tokeninfoContextFactory(r)
	user, err := c.CurrentOAuthUser("scope.one")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if user.Email != tokeinfoEmail {
		t.Errorf("expected email %q, got %q", tokeinfoEmail, user.ID)
	}
}
