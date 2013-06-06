package endpoints

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"testing"
	"time"

	"appengine/memcache"

	mc_pb "appengine_internal/memcache"
	fetch_pb "appengine_internal/urlfetch"
	user_pb "appengine_internal/user"
	"code.google.com/p/goprotobuf/proto"

	tu "github.com/crhym3/aegot/testutils"
)

func TestGetToken(t *testing.T) {
	tts := []struct {
		header, value, expected string
	}{
		{"Authorization", "Bearer token", "token"},
		{"Authorization", "bearer foo", "foo"},
		{"authorization", "bearer bar", "bar"},
		{"Authorization", "OAuth baz", "baz"},
		{"authorization", "oauth xxx", "xxx"},
		{"Authorization", "Bearer", ""},
		{"authorization", "Bearer  ", ""},
		{"Authorization", "", ""},
		{"X-Other-Header", "Bearer token", ""},
		{"x-header", "Bearer token", ""},
		{"", "", ""},
	}
	for i, tt := range tts {
		h := make(http.Header)
		if tt.header != "" {
			h.Set(tt.header, tt.value)
		}
		r := &http.Request{Header: h}

		out := getToken(r)
		if out != tt.expected {
			t.Errorf("%d: expected %q, got %q", i, tt.expected, out)
		}
	}
}

func TestGetMaxAge(t *testing.T) {
	verifyTT(t,
		getMaxAge("max-age=86400"), 86400,
		getMaxAge("max-age = 7200, must-revalidate"), 7200,
		getMaxAge("public, max-age= 3600"), 3600,
		getMaxAge("max-age=-100"), 0,
		getMaxAge("max-age = 0a1b, must-revalidate"), 0,
		getMaxAge("public, max-age= short"), 0,
		getMaxAge("s-maxage=86400"), 0,
		getMaxAge("max=86400"), 0,
		getMaxAge("public"), 0,
		getMaxAge(""), 0,
	)
}

func TestGetCertExpirationTime(t *testing.T) {
	tts := []struct {
		cacheControl, age string
		expected          time.Duration
	}{
		{"max-age=3600", "600", 3000 * time.Second},
		{"max-age=600", "", 0},
		{"max-age=300", "301", 0},
		{"max-age=0", "0", 0},
		{"", "600", 0},
		{"", "", 0},
	}

	for i, tt := range tts {
		h := make(http.Header)
		h.Set("cache-control", tt.cacheControl)
		h.Set("age", tt.age)

		if out := getCertExpirationTime(h); out != tt.expected {
			t.Errorf("%d: expected %d, got %d", i, tt.expected, out)
		}
	}
}

func TestAddBase64Pad(t *testing.T) {
	verifyTT(t,
		addBase64Pad("12"), "12==",
		addBase64Pad("123"), "123=",
		addBase64Pad("1234"), "1234",
		addBase64Pad("12345"), "12345",
		addBase64Pad(""), "")
}

func TestBase64ToBig(t *testing.T) {
	tts := []struct {
		in       string
		error    bool
		expected *big.Int
	}{
		{"MTI=", false, new(big.Int).SetBytes([]byte("12"))},
		{"MTI", false, new(big.Int).SetBytes([]byte("12"))},
		{"", false, big.NewInt(0)},
		{"    ", true, nil},
	}

	for i, tt := range tts {
		out, err := base64ToBig(tt.in)
		switch {
		case err == nil && !tt.error && tt.expected.Cmp(out) != 0:
			t.Errorf("%d: expected %v, got %v", i, tt.expected, out)
		case err != nil && !tt.error:
			t.Errorf("%d: expected %v, got error %v", i, tt.expected, err)
		case err == nil && tt.error:
			t.Errorf("%d: expected error, got %v", i, out)
		}
	}
}

func TestZeroPad(t *testing.T) {
	padded := zeroPad([]byte{1, 2, 3}, 5)
	expected := []byte{0, 0, 1, 2, 3}
	if !bytes.Equal(padded, expected) {
		t.Errorf("Expected %#v, got %#v", expected, padded)
	}
}

func TestContains(t *testing.T) {
	tts := []struct {
		list     []string
		val      string
		expected bool
	}{
		{[]string{"test"}, "test", true},
		{[]string{"one", "test", "two"}, "test", true},
		{[]string{"test"}, "xxx", false},
		{[]string{"xxx"}, "test", false},
		{[]string{}, "", false},
	}

	for i, tt := range tts {
		res := contains(tt.list, tt.val)
		if res != tt.expected {
			t.Errorf("%d: expected contains(%#v, %q) == %v, got %v",
				i, tt.list, tt.val, tt.expected, res)
		}
	}
}

func TestGetCachedCertsCacheHit(t *testing.T) {
	var cacheValue []byte
	mcGetStub := func(in, out proto.Message, _ *tu.RpcCallOptions) error {
		req := in.(*mc_pb.MemcacheGetRequest)
		if req.GetNameSpace() != certNamespace {
			t.Errorf("memcache: expected %q ns, got %q",
				req.GetNameSpace(), certNamespace)
		}

		item := &mc_pb.MemcacheGetResponse_Item{
			Key:   req.Key[0],
			Value: cacheValue,
		}
		resp := out.(*mc_pb.MemcacheGetResponse)
		resp.Item = []*mc_pb.MemcacheGetResponse_Item{item}
		return nil
	}
	defer tu.RegisterAPIOverride("memcache", "Get", mcGetStub)()
	r, deleteAppengineContext := tu.NewTestRequest("GET", "/", nil)
	defer deleteAppengineContext()

	tts := []struct {
		cacheValue string
		expected   *certsList
	}{
		{"", nil},
		{"{}", &certsList{}},
		{`{"keyvalues": [{}]}`, &certsList{[]*certInfo{{}}}},
		{`{"keyvalues": [
	    	{"algorithm": "RS256",
	    	 "exponent": "123",
	    	 "keyid": "some-id",
	    	 "modulus": "123"} ]}`,
			&certsList{[]*certInfo{{"RS256", "123", "some-id", "123"}}}},
	}
	for i, tt := range tts {
		cacheValue = []byte(tt.cacheValue)
		out, err := getCachedCerts(NewContext(r))
		switch {
		case err != nil && tt.expected != nil:
			t.Errorf("%d: didn't expect error %v", i, err)
		case err == nil && tt.expected == nil:
			t.Errorf("%d: expected error, got %#v", i, out)
		case err == nil && tt.expected != nil:
			assertEquals(t, i, out, tt.expected)
		}
	}
}

func TestGetCachedCertsCacheMiss(t *testing.T) {
	type tt struct {
		mcGetErr, mcSetErr, fetchErr error
		respStatus                   int32
		respContent                  []byte
		cacheControl, age            string

		expected        *certsList
		shouldCallMcSet bool
	}
	var (
		i           int
		currTT      *tt
		mcSetCalled bool
	)

	mcGetStub := func(in, out proto.Message, _ *tu.RpcCallOptions) error {
		return currTT.mcGetErr
	}
	mcSetStub := func(in, out proto.Message, _ *tu.RpcCallOptions) error {
		mcSetCalled = true
		req := in.(*mc_pb.MemcacheSetRequest)
		verifyTT(t,
			req.GetNameSpace(), certNamespace,
			string(req.GetItem()[0].Value), string(currTT.respContent))
		return currTT.mcSetErr
	}
	fetchStub := func(in, out proto.Message, _ *tu.RpcCallOptions) error {
		resp := out.(*fetch_pb.URLFetchResponse)
		resp.StatusCode = proto.Int32(currTT.respStatus)
		resp.Content = currTT.respContent
		resp.Header = []*fetch_pb.URLFetchResponse_Header{
			{
				Key:   proto.String("cache-control"),
				Value: proto.String(currTT.cacheControl),
			},
			{
				Key:   proto.String("age"),
				Value: proto.String(currTT.age),
			},
		}
		return currTT.fetchErr
	}
	defer tu.RegisterAPIOverride("memcache", "Get", mcGetStub)()
	defer tu.RegisterAPIOverride("memcache", "Set", mcSetStub)()
	defer tu.RegisterAPIOverride("urlfetch", "Fetch", fetchStub)()
	r, deleteAppengineContext := tu.NewTestRequest("GET", "/", nil)
	defer deleteAppengineContext()

	tts := []*tt{
		// mcGet, mcSet, fetch err, http status, content,
		// cache, age, expected, should mcSet?
		{memcache.ErrCacheMiss, nil, nil, 200, []byte(`{"keyvalues":null}`),
			"max-age=3600", "600", &certsList{}, true},
		{memcache.ErrServerError, nil, nil, 200, []byte(`{"keyvalues":null}`),
			"max-age=3600", "600", &certsList{}, false},
		{memcache.ErrCacheMiss, memcache.ErrServerError, nil, 200,
			[]byte(`{"keyvalues":null}`),
			"max-age=3600", "600", &certsList{}, true},
		{memcache.ErrCacheMiss, nil, errors.New("fetch RPC error"), 0, nil,
			"", "", nil, false},
		{memcache.ErrCacheMiss, nil, nil, 400, []byte(""),
			"", "", nil, false},
		{memcache.ErrCacheMiss, nil, nil, 200, []byte(`{"keyvalues":null}`),
			"", "", &certsList{}, false},
	}

	c := NewContext(r)

	for i, currTT = range tts {
		mcSetCalled = false
		out, err := getCachedCerts(c)
		switch {
		case err != nil && currTT.expected != nil:
			t.Errorf("%d: unexpected error: %v", i, err)
		case err == nil && currTT.expected == nil:
			t.Errorf("%d: expected error, got %#v", i, out)
		default:
			assertEquals(t, i, out, currTT.expected)
			if currTT.shouldCallMcSet != mcSetCalled {
				t.Errorf("%d: mc set called? %v, expected: %v",
					i, mcSetCalled, currTT.shouldCallMcSet)
			}
		}
	}
}

func TestCurrentBearerTokenUser(t *testing.T) {
	var (
		validScope    = "valid.scope"
		validClientId = "my-client-id"

		email      = "dude@gmail.com"
		userId     = "12345"
		authDomain = "gmail.com"
		isAdmin    = false

		empty = []string{}
	)

	getOAuthUser := func(in, out proto.Message, _ *tu.RpcCallOptions) error {
		scope := in.(*user_pb.GetOAuthUserRequest).GetScope()
		if scope != validScope {
			return fmt.Errorf("Invalid scope: %q", scope)
		}
		resp := out.(*user_pb.GetOAuthUserResponse)
		resp.ClientId = proto.String(validClientId)
		resp.Email = proto.String(email)
		resp.UserId = proto.String(userId)
		resp.AuthDomain = proto.String(authDomain)
		resp.IsAdmin = proto.Bool(isAdmin)
		return nil
	}
	unregister := tu.RegisterAPIOverride("user", "GetOAuthUser", getOAuthUser)
	defer unregister()

	req, deleteAppengineCtx := tu.NewTestRequest("GET", "/", nil)
	defer deleteAppengineCtx()
	c := cachingContextFactory(req)

	tt := []*struct {
		scopes    []string
		clientIDs []string
		success   bool
	}{
		{empty, empty, false},
		{empty, []string{validClientId}, false},
		{[]string{validScope}, empty, false},
		{[]string{validScope}, []string{validClientId}, true},
		{[]string{"a", validScope, "b"}, []string{"c", validClientId, "d"}, true},
	}
	for _, elem := range tt {
		user, err := CurrentBearerTokenUser(c, elem.scopes, elem.clientIDs)
		switch {
		case elem.success && (err != nil || user == nil):
			t.Errorf("Did not expect the call to fail with "+
				"scopes=%v ids=%v. User: %+v, Error: %q",
				elem.scopes, elem.clientIDs, err, user)
		case !elem.success && err == nil:
			t.Errorf("Expected an error, got nil: scopes=%v ids=%v",
				elem.scopes, elem.clientIDs)
		}
	}

	scopes := []string{validScope}
	clientIDs := []string{validClientId}
	user, _ := CurrentBearerTokenUser(c, scopes, clientIDs)
	const failMsg = "Expected %q, got %q"
	if user.ID != userId {
		t.Errorf(failMsg, userId, user.ID)
	}
	if user.Email != email {
		t.Errorf(failMsg, email, user.Email)
	}
	if user.AuthDomain != authDomain {
		t.Errorf(failMsg, authDomain, user.AuthDomain)
	}
	if user.Admin != isAdmin {
		t.Errorf(failMsg, isAdmin, user.Admin)
	}
}

func TestCurrentUser(t *testing.T) {
	const (
		clientId    = "my-client-id"
		bearerEmail = "bearer@example.org"
		validScope  = "valid.scope"
	)

	getOAuthRPC := func(in, out proto.Message, _ *tu.RpcCallOptions) error {
		scope := in.(*user_pb.GetOAuthUserRequest).GetScope()
		if scope != validScope && scope != EmailScope {
			return fmt.Errorf("Invalid scope: %q", scope)
		}
		resp := out.(*user_pb.GetOAuthUserResponse)
		resp.ClientId = proto.String(clientId)
		resp.Email = proto.String(bearerEmail)
		resp.AuthDomain = proto.String("example.org")
		resp.UserId = proto.String("12345")
		return nil
	}
	defer tu.RegisterAPIOverride("user", "GetOAuthUser", getOAuthRPC)()

	// stubs to make fake JWT token validations pass
	defer stubMemcacheGetCerts()() // in jwt_test.go
	origCurrentUTC := currentUTC
	defer func() {
		currentUTC = origCurrentUTC
	}()
	currentUTC = func() time.Time {
		return jwtValidTokenTime
	}

	jwtStr, jwt := jwtValidTokenString, jwtValidTokenObject
	tts := []struct {
		token                        string
		scopes, audiences, clientIDs []string
		expectedEmail                string
	}{
		// success
		{jwtStr, []string{EmailScope}, []string{jwt.Audience}, []string{jwt.ClientID}, jwt.Email},
		{"ya29.token", []string{EmailScope}, []string{clientId}, []string{clientId}, bearerEmail},
		{"ya29.token", []string{EmailScope, validScope}, []string{clientId}, []string{clientId}, bearerEmail},
		{"1/token", []string{validScope}, []string{clientId}, []string{clientId}, bearerEmail},

		// failure
		{jwtStr, []string{EmailScope}, []string{"other-client"}, []string{"other-client"}, ""},
		{"some.invalid.jwt", []string{EmailScope}, []string{jwt.Audience}, []string{jwt.ClientID}, ""},
		{"", []string{validScope}, []string{clientId}, []string{clientId}, ""},
		{"ya29.invalid", []string{"invalid.scope"}, []string{clientId}, []string{clientId}, ""},

		{"doesn't matter", nil, []string{clientId}, []string{clientId}, ""},
		{"doesn't matter", []string{EmailScope}, nil, []string{clientId}, ""},
		{"doesn't matter", []string{EmailScope}, []string{clientId}, nil, ""},
	}

	for i, tt := range tts {
		req, deleteCtx := tu.NewTestRequest("GET", "/", nil)
		defer deleteCtx()
		c := cachingContextFactory(req)
		if tt.token != "" {
			req.Header.Set("authorization", "oauth "+tt.token)
		}

		user, err := CurrentUser(c, tt.scopes, tt.audiences, tt.clientIDs)
		switch {
		case tt.expectedEmail == "" && err == nil:
			t.Errorf("%d: expected error, got %#v", i, user)
		case tt.expectedEmail != "" && user == nil:
			t.Errorf("%d: expected user object, got nil (%v)", i, err)
		case tt.expectedEmail != "" && tt.expectedEmail != user.Email:
			t.Errorf("%d: expected %q, got %q", i, tt.expectedEmail, user.Email)
		}
	}
}
