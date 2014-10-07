package endpoints

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"strings"
	"testing"
	"time"

	"appengine"
	"appengine/aetest"
	"appengine/memcache"
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
	origTransport := httpTransportFactory
	defer func() { httpTransportFactory = origTransport }()
	httpTransportFactory = func(c appengine.Context) http.RoundTripper {
		return newTestRoundTripper()
	}

	req, _, closer := newTestRequest(t, "GET", "/", nil)
	defer closer()
	nc, err := appengine.Namespace(appengine.NewContext(req), certNamespace)
	if err != nil {
		t.Fatal(err)
	}

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
	ec := NewContext(req)
	for i, tt := range tts {
		item := &memcache.Item{Key: DefaultCertUri, Value: []byte(tt.cacheValue)}
		if err := memcache.Set(nc, item); err != nil {
			t.Fatal(err)
		}
		out, err := getCachedCerts(ec)
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
	rt := newTestRoundTripper()
	origTransport := httpTransportFactory
	defer func() { httpTransportFactory = origTransport }()
	httpTransportFactory = func(c appengine.Context) http.RoundTripper {
		return rt
	}

	req, _, closer := newTestRequest(t, "GET", "/", nil)
	defer closer()
	nc, err := appengine.Namespace(appengine.NewContext(req), certNamespace)
	if err != nil {
		t.Fatal(err)
	}
	ec := NewContext(req)

	tts := []*struct {
		respStatus                     int
		respContent, cacheControl, age string
		expected                       *certsList
		shouldCache                    bool
	}{
		{200, `{"keyvalues":null}`, "max-age=3600", "600", &certsList{}, true},
		{-1, "", "", "", nil, false},
		{400, "", "", "", nil, false},
		{200, `{"keyvalues":null}`, "", "", &certsList{}, false},
	}

	for i, tt := range tts {
		if tt.respStatus > 0 {
			resp := &http.Response{
				Status:     fmt.Sprintf("%d", tt.respStatus),
				StatusCode: tt.respStatus,
				Body:       ioutil.NopCloser(strings.NewReader(tt.respContent)),
				Header:     make(http.Header),
			}
			resp.Header.Set("cache-control", tt.cacheControl)
			resp.Header.Set("age", tt.age)
			rt.Add(resp)
		}
		memcache.Delete(nc, DefaultCertUri)

		out, err := getCachedCerts(ec)
		switch {
		case err != nil && tt.expected != nil:
			t.Errorf("%d: unexpected error: %v", i, err)
		case err == nil && tt.expected == nil:
			t.Errorf("%d: expected error, got %#v", i, out)
		default:
			assertEquals(t, i, out, tt.expected)
			if !tt.shouldCache {
				continue
			}
			item, err := memcache.Get(nc, DefaultCertUri)
			if err != nil {
				t.Errorf("%d: expected cache, got %v", err)
				continue
			}
			cert := string(item.Value)
			if tt.respContent != cert {
				t.Errorf("%d: expected cache: %s, got: %s", i, tt.respContent, cert)
			}
		}
	}
}

func TestCurrentBearerTokenUser(t *testing.T) {
	var empty = []string{}
	const (
		// Default values from user_service_stub.py of dev_appserver2.
		validScope    = "valid.scope"
		validClientId = "123456789.apps.googleusercontent.com"
		email         = "example@example.com"
		userId        = "0"
		authDomain    = "gmail.com"
		isAdmin       = false
	)

	inst, err := aetest.NewInstance(nil)
	if err != nil {
		t.Fatalf("Failed to create instance: %v", err)
	}
	defer inst.Close()

	tts := []*struct {
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
	for _, tt := range tts {
		r, err := inst.NewRequest("GET", "/", nil)
		if err != nil {
			t.Fatalf("Failed to create req: %v", err)
		}
		c := cachingContextFactory(r)
		user, err := CurrentBearerTokenUser(c, tt.scopes, tt.clientIDs)
		switch {
		case tt.success && (err != nil || user == nil):
			t.Errorf("Did not expect the call to fail with "+
				"scopes=%v ids=%v. User: %+v, Error: %q",
				tt.scopes, tt.clientIDs, user, err)
		case !tt.success && err == nil:
			t.Errorf("Expected an error, got nil: scopes=%v ids=%v",
				tt.scopes, tt.clientIDs)
		}
	}

	r, err := inst.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatalf("Failed to create req: %v", err)
	}
	c := cachingContextFactory(r)

	scopes := []string{validScope}
	clientIDs := []string{validClientId}
	user, err := CurrentBearerTokenUser(c, scopes, clientIDs)
	if err != nil {
		t.Fatalf("Error getting user with scopes=%v clientIDs=%v - %v",
			scopes, clientIDs, err)
	}

	if user.ID != userId {
		t.Errorf("Expected %q, got %q", userId, user.ID)
	}
	if user.Email != email {
		t.Errorf("Expected %q, got %q", email, user.Email)
	}
	if user.AuthDomain != authDomain {
		t.Errorf("Expected %q, got %q", authDomain, user.AuthDomain)
	}
	if user.Admin != isAdmin {
		t.Errorf("Expected %q, got %q", isAdmin, user.Admin)
	}
}

func TestCurrentUser(t *testing.T) {
	const (
		// Default values from user_service_stub.py of dev_appserver2.
		clientId    = "123456789.apps.googleusercontent.com"
		bearerEmail = "example@example.com"
		validScope  = "valid.scope"
	)

	inst, err := aetest.NewInstance(nil)
	if err != nil {
		t.Fatalf("Failed to create instance: %v", err)
	}
	defer inst.Close()

	req, err := inst.NewRequest("GET", "/", nil)
	nc, err := appengine.Namespace(appengine.NewContext(req), certNamespace)
	if err != nil {
		t.Fatal(err)
	}
	// googCerts are provided in jwt_test.go
	item := &memcache.Item{Key: DefaultCertUri, Value: []byte(googCerts)}
	if err := memcache.Set(nc, item); err != nil {
		t.Fatal(err)
	}

	origCurrentUTC := currentUTC
	defer func() { currentUTC = origCurrentUTC }()
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
		// The following test is commented for now because default implementation
		// of UserServiceStub in dev_appserver2 allows any scope.
		// TODO: figure out how to test this.
		//{"ya29.invalid", []string{"invalid.scope"}, []string{clientId}, []string{clientId}, ""},

		{"doesn't matter", nil, []string{clientId}, []string{clientId}, ""},
		{"doesn't matter", []string{EmailScope}, nil, []string{clientId}, ""},
		{"doesn't matter", []string{EmailScope}, []string{clientId}, nil, ""},
	}

	for i, tt := range tts {
		r, err := inst.NewRequest("GET", "/", nil)
		c := cachingContextFactory(r)
		if tt.token != "" {
			r.Header.Set("authorization", "oauth "+tt.token)
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
