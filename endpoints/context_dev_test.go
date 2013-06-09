package endpoints

import (
	"errors"
	"testing"

	mc_pb "appengine_internal/memcache"
	fetch_pb "appengine_internal/urlfetch"
	"code.google.com/p/goprotobuf/proto"

	tu "github.com/crhym3/aegot/testutils"
)

const (
	tokeinfoUserId = "12345"
	tokeinfoEmail  = "dude@gmail.com"
)

var tokeninfoBytes = []byte(`{
	"issued_to": "my-client-id",
	"audience": "my-client-id",
	"user_id": "` + tokeinfoUserId + `",
	"scope": "scope.one scope.two",
	"expires_in": 3600,
	"email": "` + tokeinfoEmail + `",
	"verified_email": true,
	"access_type": "online"
}`)

func TestTokeninfoContextCurrentOAuthClientID(t *testing.T) {
	const token = "some_token"

	type test struct {
		token, scope, clientId string
		httpStatus             int32
		content                []byte
		fetchErr               error
	}

	var currTT *test

	mcGetStub := func(in, out proto.Message, _ *tu.RpcCallOptions) error {
		req := in.(*mc_pb.MemcacheGetRequest)
		if req.GetNameSpace() != tokeninfoContextNS {
			t.Errorf("memcache get: expected %q ns, got %q",
				req.GetNameSpace(), tokeninfoContextNS)
		}
		if key := string(req.Key[0]); key != token {
			t.Errorf("memcache get: expected memcache key %q, got %q", token, key)
		}
		return nil
	}
	defer tu.RegisterAPIOverride("memcache", "Get", mcGetStub)()

	mcSetStub := func(in, out proto.Message, _ *tu.RpcCallOptions) error {
		req := in.(*mc_pb.MemcacheSetRequest)
		if key := string(req.Item[0].Key); key != token {
			t.Errorf("memcache set: expected key %q, got %q", token, key)
		}
		return nil
	}
	defer tu.RegisterAPIOverride("memcache", "Set", mcSetStub)()

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
		// scope, clientId, httpStatus, content, fetchErr
		{token, "scope.two", "my-client-id", 200, tokeninfoBytes, nil},
		{token, "scope.one", "my-client-id", 200, tokeninfoBytes, nil},
		{token, "invalid.scope", "", 200, tokeninfoBytes, nil},
		{token, "scope.one", "", 400, []byte("{}"), nil},
		{token, "scope.one", "", 200, []byte(""), nil},
		{token, "scope.one", "", -1, nil, errors.New("Fake urlfetch error")},
		{"", "scope.one", "", 200, tokeninfoBytes, nil},
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
		resp.Content = tokeninfoBytes
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
	if user.ID != tokeinfoUserId {
		t.Errorf("expected user ID %q, got %q", tokeinfoUserId, user.ID)
	}
	if user.Email != tokeinfoEmail {
		t.Errorf("expected email %q, got %q", tokeinfoEmail, user.ID)
	}
}
