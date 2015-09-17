package endpoints

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

const (
	dummyClientID = "dummy-client-id"
	dummyScope1   = "http://dummy.scope.1"
	dummyScope2   = "http://dummy.scope.2"
	dummyAudience = "people"
)

var (
	emptySlice = []string{}
	clientIDs  = []string{dummyClientID}
	scopes     = []string{dummyScope1, dummyScope2}
	audiences  = []string{dummyAudience}
)

type canMarshal struct {
	name string
}

func (m *canMarshal) MarshalJSON() ([]byte, error) {
	return []byte("Hello, " + m.name), nil
}

func (m *canMarshal) UnmarshalJSON(b []byte) error {
	parts := strings.SplitN(string(b), " ", 2)
	if len(parts) > 1 {
		m.name = parts[1]
	} else {
		m.name = parts[0]
	}
	return nil
}

// make sure canMarshal type implements json.Marshaler and json.Unmarshaler
// interfaces
var _ = json.Marshaler((*canMarshal)(nil))
var _ = json.Unmarshaler((*canMarshal)(nil))

type DummyMsg struct {
	String    string   `json:"str" endpoints:"req,desc=A string field"`
	Int       int      `json:"i" endpoints:"min=-200,max=200,d=-100"`
	Uint      uint     `endpoints:"min=0,max=100"`
	Int64     int64    `endpoints:"d=123"`
	Uint64    uint64   `endpoints:"d=123"`
	Float32   float32  `endpoints:"d=123.456"`
	Float64   float64  `endpoints:"d=123.456"`
	BoolField bool     `json:"bool_field" endpoints:"d=true"`
	Pstring   *string  `json:"pstring"`
	Pint      *int     `json:"pint"`
	Puint     *uint    `json:"puint"`
	Pint64    *int64   `json:"pint64"`
	Puint64   *uint64  `json:"puint64"`
	Pfloat32  *float32 `json:"pfloat32"`
	Pfloat64  *float64 `json:"pfloat64"`
	PBool     *bool    `json:"pbool"`
	Bytes     []byte
	Internal  string `json:"-"`
	Marshal   *canMarshal
}

type DummySubMsg struct {
	Simple  string    `json:"simple" endpoints:"d=Hello gopher"`
	Message *DummyMsg `json:"msg"`
}

type DummyListReq struct {
	Limit  int         `json:"limit" endpoints:"d=10,max=100"`
	Cursor *canMarshal `json:"cursor"`
}

type DummyListMsg struct {
	Messages []*DummyMsg `json:"items"`
}

type DummyService struct {
}

func (s *DummyService) Post(*http.Request, *DummyMsg, *DummySubMsg) error {
	return nil
}

func (s *DummyService) PutAuth(*http.Request, *DummyMsg, *VoidMessage) error {
	return nil
}

func (s *DummyService) GetSub(*http.Request, *DummySubMsg, *DummyMsg) error {
	return nil
}

func (s *DummyService) GetList(*http.Request, *DummyListReq, *DummyListMsg) error {
	return nil
}

// createDescriptor creates APIDescriptor for DummyService.
func createDescriptor(t *testing.T) *APIDescriptor {
	dummy := &DummyService{}
	server := NewServer("")
	s, err := server.RegisterService(dummy, "Dummy", "v1", "A service", true)
	if err != nil {
		t.Fatalf("createDescriptor: error registering service: %v", err)
	}

	info := s.MethodByName("Post").Info()
	info.Name, info.Path, info.HTTPMethod, info.Desc =
		"post", "post/{i}/{bool_field}/{Float64}", "POST", "A POST method"

	info = s.MethodByName("PutAuth").Info()
	info.Name, info.Path, info.HTTPMethod, info.Desc =
		"auth", "auth", "PUT", "Method with auth"
	info.ClientIds, info.Scopes, info.Audiences =
		clientIDs, scopes, audiences

	info = s.MethodByName("GetSub").Info()
	info.Name, info.Path, info.HTTPMethod, info.Desc =
		"sub.sub", "sub/{simple}/{msg.i}/{msg.str}", "GET", "With substruct"

	info = s.MethodByName("GetList").Info()
	info.Name, info.Path, info.HTTPMethod, info.Desc =
		"list", "list", "GET", "Messages list"

	d := &APIDescriptor{}
	if err := s.APIDescriptor(d, "testhost:1234"); err != nil {
		t.Fatalf("createDescriptor: error creating descriptor: %v", err)
	}
	return d
}

func TestAPIDescriptor(t *testing.T) {
	d := createDescriptor(t)
	verifyPairs(t,
		d.Extends, "thirdParty.api",
		d.Root, "https://testhost:1234/_ah/api",
		d.Name, "dummy",
		d.Version, "v1",
		d.Default, true,
		d.Adapter.Bns, "https://testhost:1234/_ah/spi",
		d.Adapter.Type, "lily",
		len(d.Methods), 4,
		len(d.Descriptor.Methods), 4,
		len(d.Descriptor.Schemas), 3,
		d.Desc, "A service",
	)
}

// ---------------------------------------------------------------------------
// $METHOD_MAP

func TestAPIPostMethod(t *testing.T) {
	d := createDescriptor(t)
	meth := d.Methods["dummy.post"]
	if meth == nil {
		t.Fatal("want APIMethod 'dummy.post'")
		return
	}
	verifyPairs(t,
		meth.Path, "post/{i}/{bool_field}/{Float64}",
		meth.HTTPMethod, "POST",
		meth.RosyMethod, "DummyService.Post",
		meth.Request.Body, "autoTemplate(backendRequest)",
		meth.Request.BodyName, "resource",
		meth.Response.Body, "autoTemplate(backendResponse)",
		meth.Response.BodyName, "resource",
		len(meth.Scopes), 0,
		len(meth.Audiences), 0,
		len(meth.ClientIds), 0,
		meth.Desc, "A POST method",
	)

	params := meth.Request.Params
	tts := [][]interface{}{
		{"i", "int32", true, -100, -200, 200, false, 0},
		{"bool_field", "boolean", true, true, nil, nil, false, 0},
		{"Float64", "double", true, 123.456, nil, nil, false, 0},
	}

	for _, tt := range tts {
		name := tt[0].(string)
		p := params[name]
		if p == nil {
			t.Errorf("want param %q in %v", name, params)
		}
		verifyPairs(t,
			p.Type, tt[1],
			p.Required, tt[2],
			p.Default, tt[3],
			p.Min, tt[4],
			p.Max, tt[5],
			p.Repeated, tt[6],
			len(p.Enum), tt[7],
		)
	}
	if lp, ltts := len(params), len(tts); lp != ltts {
		t.Errorf("have %d params for %q; want %d", lp, meth.RosyMethod, ltts)
	}
}

func TestAPIPutAuthMethod(t *testing.T) {
	d := createDescriptor(t)
	meth := d.Methods["dummy.auth"]
	if meth == nil {
		t.Fatal("want APIMethod 'dummy.auth'")
		return
	}
	verifyPairs(t,
		meth.HTTPMethod, "PUT",
		meth.RosyMethod, "DummyService.PutAuth",
		meth.Request.Body, "autoTemplate(backendRequest)",
		meth.Request.BodyName, "resource",
		meth.Response.Body, "empty",
		meth.Response.BodyName, "",
		meth.ClientIds, []string{dummyClientID},
		meth.Scopes, []string{dummyScope1, dummyScope2},
		meth.Audiences, []string{dummyAudience},
		len(meth.Request.Params), 0,
	)
}

func TestAPIGetSubMethod(t *testing.T) {
	d := createDescriptor(t)
	// apiname.resource.method
	meth := d.Methods["dummy.sub.sub"]
	if meth == nil {
		t.Fatal("want APIMethod 'dummy.sub.sub'")
	}
	verifyPairs(t,
		meth.Path, "sub/{simple}/{msg.i}/{msg.str}",
		meth.HTTPMethod, "GET",
		meth.RosyMethod, "DummyService.GetSub",
		meth.Request.Body, "empty",
		meth.Request.BodyName, "",
		meth.Response.Body, "autoTemplate(backendResponse)",
		meth.Response.BodyName, "resource",
		len(meth.Scopes), 0,
		len(meth.Audiences), 0,
		len(meth.ClientIds), 0,
	)

	params := meth.Request.Params
	tts := [][]interface{}{
		{"simple", "string", false, "Hello gopher", nil, nil, false, 0},
		{"msg.i", "int32", false, -100, -200, 200, false, 0},
		{"msg.str", "string", true, nil, nil, nil, false, 0},
		{"msg.Int64", "int64", false, int64(123), nil, nil, false, 0},
		{"msg.Uint", "uint32", false, nil, uint32(0), uint32(100), false, 0},
		{"msg.Uint64", "uint64", false, uint64(123), nil, nil, false, 0},
		{"msg.Float32", "float", false, float32(123.456), nil, nil, false, 0},
		{"msg.Float64", "double", false, 123.456, nil, nil, false, 0},
		{"msg.bool_field", "boolean", false, true, nil, nil, false, 0},
		{"msg.pstring", "string", false, nil, nil, nil, false, 0},
		{"msg.pint", "int32", false, nil, nil, nil, false, 0},
		{"msg.puint", "uint32", false, nil, nil, nil, false, 0},
		{"msg.pint64", "int64", false, nil, nil, nil, false, 0},
		{"msg.puint64", "uint64", false, nil, nil, nil, false, 0},
		{"msg.pfloat32", "float", false, nil, nil, nil, false, 0},
		{"msg.pfloat64", "double", false, nil, nil, nil, false, 0},
		{"msg.pbool", "boolean", false, nil, nil, nil, false, 0},
		{"msg.Bytes", "bytes", false, nil, nil, nil, false, 0},
		{"msg.Marshal", "string", false, nil, nil, nil, false, 0},
	}

	for _, tt := range tts {
		name := tt[0].(string)
		p := params[name]
		if p == nil {
			t.Errorf("want param %q in %#v", name, params)
			continue
		}
		verifyPairs(t,
			p.Type, tt[1],
			p.Required, tt[2],
			p.Default, tt[3],
			p.Min, tt[4],
			p.Max, tt[5],
			p.Repeated, tt[6],
			len(p.Enum), tt[7],
		)
	}

	if lp, ltts := len(params), len(tts); lp != ltts {
		t.Errorf("have %d params for %q; want %d", lp, meth.RosyMethod, ltts)
	}
}

func TestAPIGetListMethod(t *testing.T) {
	d := createDescriptor(t)
	meth := d.Methods["dummy.list"]
	if meth == nil {
		t.Fatal("want APIMethod 'dummy.list'")
		return
	}
	verifyPairs(t,
		meth.HTTPMethod, "GET",
		meth.RosyMethod, "DummyService.GetList",
		meth.Request.Body, "empty",
		meth.Request.BodyName, "",
		meth.Response.Body, "autoTemplate(backendResponse)",
		meth.Response.BodyName, "resource",
		len(meth.Scopes), 0,
		len(meth.Audiences), 0,
		len(meth.ClientIds), 0,
	)

	params := meth.Request.Params
	tts := [][]interface{}{
		{"limit", "int32", false, 10, nil, 100, false, 0},
		{"cursor", "string", false, nil, nil, nil, false, 0},
	}

	for _, tt := range tts {
		name := tt[0].(string)
		p := params[name]
		if p == nil {
			t.Errorf("want param %q in %v", name, params)
			continue
		}
		verifyPairs(t,
			p.Type, tt[1],
			p.Required, tt[2],
			p.Default, tt[3],
			p.Min, tt[4],
			p.Max, tt[5],
			p.Repeated, tt[6],
			len(p.Enum), tt[7],
		)
	}
	if lp, ltts := len(params), len(tts); lp != ltts {
		t.Errorf("have %d params for %q; want %d", lp, meth.RosyMethod, ltts)
	}
}

func TestDuplicateMethodName(t *testing.T) {
	dummy := &DummyService{}
	server := NewServer("")
	s, err := server.RegisterService(dummy, "Dummy", "v1", "A service", true)
	if err != nil {
		t.Fatalf("error registering service: %v", err)
	}

	s.MethodByName("GetSub").Info().Name = "someMethod"
	s.MethodByName("GetList").Info().Name = "someMethod"

	d := &APIDescriptor{}
	err = s.APIDescriptor(d, "testhost:1234")
	if err == nil {
		t.Fatal("want duplicate method error")
	}
	t.Logf("dup method error: %v", err)
}

func TestDuplicateHTTPMethodPath(t *testing.T) {
	dummy := &DummyService{}
	server := NewServer("")
	s, err := server.RegisterService(dummy, "Dummy", "v1", "A service", true)
	if err != nil {
		t.Fatalf("error registering service: %s", err)
	}

	info := s.MethodByName("GetSub").Info()
	info.HTTPMethod, info.Path = "GET", "some/{param}/path"
	info = s.MethodByName("GetList").Info()
	info.HTTPMethod, info.Path = "GET", "some/{param}/path"

	d := &APIDescriptor{}
	err = s.APIDescriptor(d, "testhost:1234")
	if err == nil {
		t.Fatal("want duplicate HTTP method + path error")
	}
	t.Logf("dup method error: %v", err)
}

func TestNotDuplicateHTTPMethodPathWhenMultipleBrackets(t *testing.T) {
	// Test for https://github.com/GoogleCloudPlatform/go-endpoints/issues/74
	dummy := &DummyService{}
	server := NewServer("")
	s, err := server.RegisterService(dummy, "Dummy", "v1", "A service", true)
	if err != nil {
		t.Fatalf("error registering service: %s", err)
	}

	info := s.MethodByName("GetSub").Info()
	info.HTTPMethod, info.Path = "GET", "some/{param}"
	info = s.MethodByName("GetList").Info()
	info.HTTPMethod, info.Path = "GET", "some/{param1}/path/{param2}"

	d := &APIDescriptor{}
	err = s.APIDescriptor(d, "testhost:1234")
	if err != nil {
		t.Logf("dup method error: %v", err)
		t.Fatal("Don't want duplicate HTTP method + path error")
	}
}

func TestPrefixedSchemaName(t *testing.T) {
	const prefix = "SomePrefix"

	origSchemaNameForType := SchemaNameForType
	defer func() { SchemaNameForType = origSchemaNameForType }()
	SchemaNameForType = func(t reflect.Type) string {
		return prefix + t.Name()
	}

	d := createDescriptor(t)
	for name := range d.Descriptor.Schemas {
		if !strings.HasPrefix(name, prefix) {
			t.Errorf("HasPrefix(%q, %q) = false", name, prefix)
		}
	}

	for mname, meth := range d.Descriptor.Methods {
		if meth.Request != nil {
			if !strings.HasPrefix(meth.Request.Ref, prefix) {
				t.Errorf("HasPrefix(%q, %q) = false; request of %q",
					meth.Request.Ref, prefix, mname)
			}
		}
		if meth.Response != nil {
			if !strings.HasPrefix(meth.Response.Ref, prefix) {
				t.Errorf("HasPrefix(%q, %q) = false; response of %q",
					meth.Response.Ref, prefix, mname)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// $SCHEMA_DESCRIPTOR (SCHEMAS)

func verifySchema(t *testing.T, schemaID string, schemaProps [][]interface{}) {
	d := createDescriptor(t)
	s := d.Descriptor.Schemas[schemaID]
	if s == nil {
		t.Errorf("want %q schema", schemaID)
		return
	}

	verifyPairs(t,
		s.ID, schemaID,
		s.Type, "object",
		s.Desc, "")

	for _, tt := range schemaProps {
		name := tt[0].(string)
		p := s.Properties[name]
		if p == nil {
			t.Errorf("want %q in %#v", name, s.Properties)
			continue
		}
		verifyPairs(t,
			p.Type, tt[1],
			p.Format, tt[2],
			p.Required, tt[3],
			p.Default, tt[4],
			p.Ref, tt[5],
			p.Desc, tt[6],
		)
		if len(tt) == 7 && p.Items != nil {
			t.Errorf("want %s.Items of %s to be nil", name, s.ID)
		} else if len(tt) == 13 {
			verifyPairs(t,
				p.Items.Type, tt[7],
				p.Items.Format, tt[8],
				p.Items.Required, tt[9],
				p.Items.Default, tt[10],
				p.Items.Ref, tt[11],
				p.Items.Desc, tt[12],
			)
		}
	}

	if lp, ltts := len(s.Properties), len(schemaProps); lp != ltts {
		t.Errorf("have %d props in %q; want %d", lp, s.ID, ltts)
	}
}

func TestDummyMsgSchema(t *testing.T) {
	props := [][]interface{}{
		// name, type, format, required, default, ref, desc
		{"str", "string", "", true, nil, "", "A string field"},
		{"i", "integer", "int32", false, -100, "", ""},
		{"Uint", "integer", "uint32", false, nil, "", ""},
		{"Int64", "string", "int64", false, int64(123), "", ""},
		{"Uint64", "string", "uint64", false, uint64(123), "", ""},
		{"Float32", "number", "float", false, float32(123.456), "", ""},
		{"Float64", "number", "double", false, float64(123.456), "", ""},
		{"bool_field", "boolean", "", false, true, "", ""},
		{"pstring", "string", "", false, nil, "", ""},
		{"pint", "integer", "int32", false, nil, "", ""},
		{"puint", "integer", "uint32", false, nil, "", ""},
		{"pint64", "string", "int64", false, nil, "", ""},
		{"puint64", "string", "uint64", false, nil, "", ""},
		{"pfloat32", "number", "float", false, nil, "", ""},
		{"pfloat64", "number", "double", false, nil, "", ""},
		{"pbool", "boolean", "", false, nil, "", ""},
		{"Bytes", "string", "byte", false, nil, "", ""},
		{"Marshal", "string", "", false, nil, "", ""},
	}

	verifySchema(t, "DummyMsg", props)
}

func TestDummySubMsgSchema(t *testing.T) {
	props := [][]interface{}{
		{"simple", "string", "", false, "Hello gopher", "", ""},
		{"msg", "", "", false, nil, "DummyMsg", ""},
	}

	verifySchema(t, "DummySubMsg", props)
}

func TestDummyListMsgSchema(t *testing.T) {
	props := [][]interface{}{
		// name, type, format, required, default, ref, desc
		{"items", "array", "", false, nil, "", "",
			// item format
			"", "", false, nil, "DummyMsg", "",
		},
	}

	verifySchema(t, "DummyListMsg", props)
}

// ---------------------------------------------------------------------------
// $SCHEMA_DESCRIPTOR (METHODS)

func TestDescriptorMethods(t *testing.T) {
	d := createDescriptor(t)

	tts := []*struct {
		name, in, out string
	}{
		{"DummyService.Post", "DummyMsg", "DummySubMsg"},
		{"DummyService.PutAuth", "DummyMsg", ""},
		{"DummyService.GetSub", "", "DummyMsg"},
		{"DummyService.GetList", "", "DummyListMsg"},
	}
	for _, tt := range tts {
		meth := d.Descriptor.Methods[tt.name]
		if meth == nil {
			t.Errorf("want %q method descriptor", tt.name)
			continue
		}

		switch {
		case tt.in == "":
			if meth.Request != nil {
				t.Errorf("%s: have req %#v; want nil",
					tt.name, meth.Request)
			}
		case tt.in != "":
			if meth.Request == nil || meth.Request.Ref != tt.in {
				t.Errorf("%s: have req %#v; want %q",
					tt.name, meth.Request, tt.in)
			}
		}
		switch {
		case tt.out == "":
			if meth.Response != nil {
				t.Errorf("%s: have req %#v; want nil",
					tt.name, meth.Response)
			}
		case tt.out != "":
			if meth.Response == nil || meth.Response.Ref != tt.out {
				t.Errorf("%s: have resp %#v; want %q",
					tt.name, meth.Response, tt.out)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Types tests

func TestFieldNamesSimple(t *testing.T) {
	s := struct {
		Name       string `json:"name"`
		Age        int
		unexported string
		Internal   string `json:"-"`
		Marshal    *canMarshal
	}{}

	m := fieldNames(reflect.TypeOf(s), true)
	names := []string{"name", "Age", "Marshal"}

	for _, k := range names {
		field, exists := m[k]
		switch {
		case !exists:
			t.Errorf("want %q in %#v", k, m)
		case field == nil:
			t.Errorf("want non-nil field %q", k)
		}
	}
	if len(m) != len(names) {
		t.Errorf("have %d elements; want %d", len(m), len(names))
	}
}

func TestFieldNamesNested(t *testing.T) {
	s := struct {
		Root   string
		Nested struct {
			Param  string
			TwoPtr *struct {
				Param string `json:"param"`
			}
		} `json:"n"`
	}{}

	tts := []struct {
		flatten bool
		names   []string
	}{
		{true, []string{"Root", "n.Param", "n.TwoPtr.param"}},
		{false, []string{"Root", "n"}},
	}

	for _, tt := range tts {
		m := fieldNames(reflect.TypeOf(s), tt.flatten)
		for _, k := range tt.names {
			if _, exists := m[k]; !exists {
				t.Errorf("want %q in %#v", k, m)
			}
		}
		if len(m) != len(tt.names) {
			t.Errorf("have %d items in %#v; want %d", len(m), m, len(tt.names))
		}
	}
}

func TestFieldNamesAnonymous(t *testing.T) {
	type Inner struct {
		Msg string
	}
	outer := struct {
		Inner
		Root string
	}{}

	m := fieldNames(reflect.TypeOf(outer), false)

	if len(m) != 2 {
		t.Fatalf("have %d fields; want 2", len(m))
	}

	if _, ok := m["Root"]; !ok {
		t.Errorf("want 'Root' field")
	}

	f, ok := m["Msg"]
	if !ok {
		t.Fatal("want 'Msg' field from 'Inner'")
	}
	if f.Type.Kind() != reflect.String {
		t.Errorf("have 'Msg' kind = %v; want %v", f.Type.Kind(), reflect.String)
	}
}

// ---------------------------------------------------------------------------
// Parse tests

func TestParsePath(t *testing.T) {
	const in = "one/{a}/two/{b}/three/{c.d}"
	out, _ := parsePath(in)
	want := []string{"a", "b", "c.d"}
	if !reflect.DeepEqual(out, want) {
		t.Errorf("parsePath(%v) = %v; want %v", in, out, want)
	}
}

func TestParseValue(t *testing.T) {
	tts := []struct {
		kind        reflect.Kind
		val         string
		want        interface{}
		shouldError bool
	}{
		{reflect.Int, "123", 123, false},
		{reflect.Int8, "123", 123, false},
		{reflect.Int16, "123", 123, false},
		{reflect.Int32, "123", 123, false},
		{reflect.Int64, "123", int64(123), false},
		{reflect.Uint, "123", uint32(123), false},
		{reflect.Uint8, "123", uint32(123), false},
		{reflect.Uint16, "123", uint32(123), false},
		{reflect.Uint32, "123", uint32(123), false},
		{reflect.Uint64, "123", uint64(123), false},
		{reflect.Float32, "123", float32(123), false},
		{reflect.Float64, "123", float64(123), false},
		{reflect.Bool, "true", true, false},
		{reflect.String, "hello", "hello", false},

		{reflect.Int, "", nil, false},
		{reflect.Struct, "{}", nil, true},
		{reflect.Float32, "x", nil, true},
	}

	for i, tt := range tts {
		out, err := parseValue(tt.val, tt.kind)
		switch {
		case err == nil && !tt.shouldError && out != tt.want:
			t.Errorf("%d: parseValue(%q) = %v (%T); want %v (%T)",
				i, tt.val, out, out, tt.want, tt.want)
		case err == nil && tt.shouldError:
			t.Errorf("%d: parseValue(%q) = %#v; want error", i, tt.val, out)
		case err != nil && !tt.shouldError:
			t.Errorf("%d: parseValue(%q) = %v", i, tt.val, err)
		}
	}
}

func TestParseTag(t *testing.T) {
	type s struct {
		Empty   string
		Ignored string `endpoints:"req,ignored_part,desc=Some field"`
		Opt     int    `endpoints:"d=123,min=1,max=200,desc=Int field"`
		Invalid uint   `endpoints:"req,d=100"`
	}

	testFields := []struct {
		name string
		tag  *endpointsTag
	}{
		{"Empty", &endpointsTag{false, "", "", "", ""}},
		{"Ignored", &endpointsTag{true, "", "", "", "Some field"}},
		{"Opt", &endpointsTag{false, "123", "1", "200", "Int field"}},
		{"Invalid", nil},
	}

	typ := reflect.TypeOf(s{})
	for i, tf := range testFields {
		field, _ := typ.FieldByName(tf.name)
		parsed, err := parseTag(field.Tag)
		switch {
		case err != nil && tf.tag != nil:
			t.Errorf("%d: parseTag(%q) = %v; want %v", i, field.Tag, err, tf.tag)
		case err == nil && tf.tag != nil && !reflect.DeepEqual(parsed, tf.tag):
			t.Errorf("%d: parseTag(%q) = %#+v; want %#+v",
				i, field.Tag, parsed, tf.tag)
		case err == nil && tf.tag == nil:
			t.Errorf("%d: parseTag(%q) = %#v; want error", i, field.Tag, parsed)
		}
	}
}
