package endpoints

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"appengine"
	"appengine/aetest"
	"appengine/memcache"
)

func TestGetToken(t *testing.T) {
	tts := []struct {
		header, value, want string
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
		if out != tt.want {
			t.Errorf("%d: getToken(%v) = %q; want %q", i, h, out, tt.want)
		}
	}
}

func TestGetMaxAge(t *testing.T) {
	verifyPairs(t,
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
		want              time.Duration
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

		if out := getCertExpirationTime(h); out != tt.want {
			t.Errorf("%d: getCertExpirationTime(%v) = %d; want %d", i, h, out, tt.want)
		}
	}
}

func TestAddBase64Pad(t *testing.T) {
	verifyPairs(t,
		addBase64Pad("12"), "12==",
		addBase64Pad("123"), "123=",
		addBase64Pad("1234"), "1234",
		addBase64Pad("12345"), "12345",
		addBase64Pad(""), "")
}

func TestBase64ToBig(t *testing.T) {
	tts := []struct {
		in    string
		want  *big.Int
		error bool
	}{
		{"MTI=", new(big.Int).SetBytes([]byte("12")), false},
		{"MTI", new(big.Int).SetBytes([]byte("12")), false},
		{"", big.NewInt(0), false},
		{"    ", nil, true},
	}

	for i, tt := range tts {
		out, err := base64ToBig(tt.in)
		switch {
		case err == nil && !tt.error && tt.want.Cmp(out) != 0:
			t.Errorf("%d: base64ToBig(%q) = %v; want %v", i, tt.in, out, tt.want)
		case err != nil && !tt.error:
			t.Errorf("%d: base64ToBig(%q) = %v; want %v", i, tt.in, err, tt.want)
		case err == nil && tt.error:
			t.Errorf("%d: base64ToBig(%q) = %v; want error", i, tt.in, out)
		}
	}
}

func TestZeroPad(t *testing.T) {
	in := []byte{1, 2, 3}
	padded := zeroPad(in, 5)
	want := []byte{0, 0, 1, 2, 3}
	if !bytes.Equal(padded, want) {
		t.Errorf("zeroPad(%#v, 5) = %#v; want %#v", in, padded, want)
	}
}

func TestContains(t *testing.T) {
	tts := []struct {
		list []string
		val  string
		want bool
	}{
		{[]string{"test"}, "test", true},
		{[]string{"one", "test", "two"}, "test", true},
		{[]string{"test"}, "xxx", false},
		{[]string{"xxx"}, "test", false},
		{[]string{}, "", false},
	}

	for i, tt := range tts {
		res := contains(tt.list, tt.val)
		if res != tt.want {
			t.Errorf("%d: contains(%#v, %q) = %v; want %v",
				i, tt.list, tt.val, res, tt.want)
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
		want       *certsList
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
		item := &memcache.Item{Key: DefaultCertURI, Value: []byte(tt.cacheValue)}
		if err := memcache.Set(nc, item); err != nil {
			t.Fatal(err)
		}
		out, err := getCachedCerts(ec)
		switch {
		case err != nil && tt.want != nil:
			t.Errorf("%d: getCachedCerts() error %v", i, err)
		case err == nil && tt.want == nil:
			t.Errorf("%d: getCachedCerts() = %#v; want error", i, out)
		case err == nil && tt.want != nil && !reflect.DeepEqual(out, tt.want):
			t.Errorf("getCachedCerts() = %#+v (%T); want %#+v (%T)",
				out, out, tt.want, tt.want)
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
		want                           *certsList
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
		memcache.Delete(nc, DefaultCertURI)

		out, err := getCachedCerts(ec)
		switch {
		case err != nil && tt.want != nil:
			t.Errorf("%d: getCachedCerts() = %v", i, err)
		case err == nil && tt.want == nil:
			t.Errorf("%d: getCachedCerts() = %#v; want error", i, out)
		default:
			if !reflect.DeepEqual(out, tt.want) {
				t.Errorf("%d: getCachedCerts() = %#v; want %#v", i, out, tt.want)
			}
			if !tt.shouldCache {
				continue
			}
			item, err := memcache.Get(nc, DefaultCertURI)
			if err != nil {
				t.Errorf("%d: memcache.Get(%q) = %v", i, DefaultCertURI, err)
				continue
			}
			cert := string(item.Value)
			if tt.respContent != cert {
				t.Errorf("%d: memcache.Get(%q) = %q; want %q",
					i, DefaultCertURI, cert, tt.respContent)
			}
		}
	}
}

func TestCurrentBearerTokenUser(t *testing.T) {
	var empty = []string{}
	const (
		// Default values from user_service_stub.py of dev_appserver2.
		validScope    = "valid.scope"
		validClientID = "123456789.apps.googleusercontent.com"
		email         = "example@example.com"
		userID        = "0"
		authDomain    = "gmail.com"
		isAdmin       = false
	)

	inst, err := aetest.NewInstance(nil)
	if err != nil {
		t.Fatalf("failed to create instance: %v", err)
	}
	defer inst.Close()

	tts := []*struct {
		scopes    []string
		clientIDs []string
		success   bool
	}{
		{empty, empty, false},
		{empty, []string{validClientID}, false},
		{[]string{validScope}, empty, false},
		{[]string{validScope}, []string{validClientID}, true},
		{[]string{"a", validScope, "b"}, []string{"c", validClientID, "d"}, true},
	}
	for i, tt := range tts {
		r, err := inst.NewRequest("GET", "/", nil)
		if err != nil {
			t.Fatalf("failed to create req: %v", err)
		}
		c := cachingContextFactory(r)
		user, err := CurrentBearerTokenUser(c, tt.scopes, tt.clientIDs)
		switch {
		case tt.success && (err != nil || user == nil):
			t.Errorf("%d: CurrentBearerTokenUser(%v, %v): err=%v, user=%+v; want ok",
				i, tt.scopes, tt.clientIDs, err, user)
		case !tt.success && err == nil:
			t.Errorf("%d: CurrentBearerTokenUser(%v, %v) = %+v; want error",
				i, tt.scopes, tt.clientIDs, user)
		}
	}

	r, err := inst.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatalf("failed to create req: %v", err)
	}
	c := cachingContextFactory(r)

	scopes := []string{validScope}
	clientIDs := []string{validClientID}
	user, err := CurrentBearerTokenUser(c, scopes, clientIDs)
	if err != nil {
		t.Fatalf("CurrentBearerTokenUser(%v, %v) = %v", scopes, clientIDs, err)
	}

	if user.ID != userID {
		t.Fatalf("CurrentBearerTokenUser(%v, %v) = %v; want ID=%v",
			scopes, clientIDs, user, userID)
	}
	if user.Email != email {
		t.Fatalf("CurrentBearerTokenUser(%v, %v) = %v; want email=%v",
			scopes, clientIDs, user, email)
	}
	if user.AuthDomain != authDomain {
		t.Fatalf("CurrentBearerTokenUser(%v, %v) = %v; want authDomain=%v",
			scopes, clientIDs, user, authDomain)
	}
	if user.Admin != isAdmin {
		t.Fatalf("CurrentBearerTokenUser(%v, %v) = %v; want isAdmin=%v",
			scopes, clientIDs, user, isAdmin)
	}
}

func TestCurrentUser(t *testing.T) {
	const (
		// Default values from user_service_stub.py of dev_appserver2.
		clientID    = "123456789.apps.googleusercontent.com"
		bearerEmail = "example@example.com"
		validScope  = "valid.scope"
	)

	inst, err := aetest.NewInstance(nil)
	if err != nil {
		t.Fatalf("failed to create instance: %v", err)
	}
	defer inst.Close()

	req, err := inst.NewRequest("GET", "/", nil)
	nc, err := appengine.Namespace(appengine.NewContext(req), certNamespace)
	if err != nil {
		t.Fatal(err)
	}
	// googCerts are provided in jwt_test.go
	item := &memcache.Item{Key: DefaultCertURI, Value: []byte(googCerts)}
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
		wantEmail                    string
	}{
		// success
		{jwtStr, []string{EmailScope}, []string{jwt.Audience}, []string{jwt.ClientID}, jwt.Email},
		{"ya29.token", []string{EmailScope}, []string{clientID}, []string{clientID}, bearerEmail},
		{"ya29.token", []string{EmailScope, validScope}, []string{clientID}, []string{clientID}, bearerEmail},
		{"1/token", []string{validScope}, []string{clientID}, []string{clientID}, bearerEmail},

		// failure
		{jwtStr, []string{EmailScope}, []string{"other-client"}, []string{"other-client"}, ""},
		{"some.invalid.jwt", []string{EmailScope}, []string{jwt.Audience}, []string{jwt.ClientID}, ""},
		{"", []string{validScope}, []string{clientID}, []string{clientID}, ""},
		// The following test is commented for now because default implementation
		// of UserServiceStub in dev_appserver2 allows any scope.
		// TODO: figure out how to test this.
		//{"ya29.invalid", []string{"invalid.scope"}, []string{clientID}, []string{clientID}, ""},

		{"doesn't matter", nil, []string{clientID}, []string{clientID}, ""},
		{"doesn't matter", []string{EmailScope}, nil, []string{clientID}, ""},
		{"doesn't matter", []string{EmailScope}, []string{clientID}, nil, ""},
	}

	for i, tt := range tts {
		r, err := inst.NewRequest("GET", "/", nil)
		c := cachingContextFactory(r)
		if tt.token != "" {
			r.Header.Set("authorization", "oauth "+tt.token)
		}

		user, err := CurrentUser(c, tt.scopes, tt.audiences, tt.clientIDs)

		switch {
		case tt.wantEmail == "" && err == nil:
			t.Errorf("%d: CurrentUser(%v, %v, %v) = %v; want error",
				i, tt.scopes, tt.audiences, tt.clientIDs, user)
		case tt.wantEmail != "" && user == nil:
			t.Errorf("%d: CurrentUser(%v, %v, %v) = %v; want email = %q",
				i, tt.scopes, tt.audiences, tt.clientIDs, err, tt.wantEmail)
		case tt.wantEmail != "" && tt.wantEmail != user.Email:
			t.Errorf("%d: CurrentUser(%v, %v, %v) = %v; want email = %q",
				i, tt.scopes, tt.audiences, tt.clientIDs, user, tt.wantEmail)
		}
	}
}
