package endpoints

import (
	"net/http"
	"testing"
)

type DummyMessage struct {
	String  string `endpoints:"required"`
	Int     int    `endpoints:"-100"`
	Uint    uint   `endpoints:"100,required"`
	Float32 float32
	Float64 float64
	Bytes   []byte
}

type DummySubstructMessage struct {
	String   string
	Message  *DummyMessage
	Messages []*DummyMessage
}

type DummyService struct {
}

func (s *DummyService) Echo(r *http.Request, req *DummyMessage, resp *DummyMessage) error {
	return nil
}

func (s *DummyService) Substruct(r *http.Request, _ *VoidMessage, resp *DummySubstructMessage) error {
	return nil
}

func createDescriptor(t *testing.T) *ApiDescriptor {
	server := NewServer("")
	dummy := &DummyService{}
	s, err := server.RegisterService(
		dummy, "Dummy", "v1", "A service", true)
	if err != nil {
		t.Error(err)
	}
	info := s.MethodByName("Echo").Info()
	info.Name, info.Path, info.HttpMethod, info.Desc =
		"echoMe", "echome/{int}/{float64}", "POST", "Echo test method"

	info = s.MethodByName("Substruct").Info()
	info.Name, info.Path, info.HttpMethod, info.Desc =
		"sub.sub", "sub/{string}", "GET", "Method with substruct"

	d := &ApiDescriptor{}
	if err := s.ApiDescriptor(d, "localhost"); err != nil {
		t.Error(err)
	}
	return d
}

func TestApiDescriptor(t *testing.T) {
	d := createDescriptor(t)
	verifyTT(t, []*ttRow{
		{d.Extends, "thirdParty.api"},
		{d.Root, "https://localhost/_ah/api"},
		{d.Name, "dummy"},
		{d.Version, "v1"},
		{d.Default, true},
		{d.Adapter.Bns, "https://localhost/_ah/spi"},
		{d.Adapter.Type, "lily"},
		{len(d.Methods), 2},
		{len(d.Descriptor.Methods), 2},
		{len(d.Descriptor.Schemas), 2},
	})
}

func TestApiEchoMethod(t *testing.T) {
	d := createDescriptor(t)
	meth := d.Methods["dummy.echoMe"]
	if meth == nil {
		t.Errorf("Expected to find ApiMethod 'dummy.echoMe'")
		return
	}
	verifyTT(t, []*ttRow{
		{meth.Path, "echome/{int}/{float64}"},
		{meth.HttpMethod, "POST"},
		{meth.RosyMethod, "DummyService.Echo"},
		{meth.Request.Body, "autoTemplate(backendRequest)"},
		{meth.Request.BodyName, "resource"},
		{meth.Response.Body, "autoTemplate(backendResponse)"},
		{meth.Response.BodyName, "resource"},
		{len(meth.Scopes), 0},
		{len(meth.Audiences), 0},
		{len(meth.ClientIds), 0},
	})

	// TODO: test meth.Request.Params map (ApiRequestParamSpec)
}

func TestApiSubstructMethod(t *testing.T) {
	d := createDescriptor(t)
	// apiname.resource.methon
	meth := d.Methods["dummy.sub.sub"]
	if meth == nil {
		t.Errorf("Expected to find ApiMethod 'dummy.substruct'")
		return
	}
	verifyTT(t, []*ttRow{
		{meth.Path, "sub/{string}"},
		{meth.HttpMethod, "GET"},
		{meth.RosyMethod, "DummyService.Substruct"},
		{meth.Request.Body, "empty"},
		{meth.Request.BodyName, ""},
		{meth.Response.Body, "autoTemplate(backendResponse)"},
		{meth.Response.BodyName, "resource"},
		{len(meth.Scopes), 0},
		{len(meth.Audiences), 0},
		{len(meth.ClientIds), 0},
	})

	// TODO: test meth.Request.Params map (ApiRequestParamSpec)
}

func TestApiDescriptorDummyMessage(t *testing.T) {
	d := createDescriptor(t)
	sch := d.Descriptor.Schemas["DummyMessage"]
	if sch == nil {
		t.Errorf("Expected to find DummyMessage schema")
	}

	// TODO: test Descriptor.Schemas and Descriptor.Methods
}

func TestParsePath(t *testing.T) {
	params, _ := parsePath("one/{a}/two/{b}/three/{c.d}")
	assertEquals(t, params, []string{"a", "b", "c.d"})
}
