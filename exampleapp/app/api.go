package app

import (
	"fmt"
	"github.com/crhym3/go-endpoints/endpoints"
	"net/http"
)

type HelloReq struct {
	Who string
}

type HelloResp struct {
	Message string `json:"message" endpoints:"required"`
	Who     string `json:"who"`
}

func SayHello(args *HelloReq, reply *HelloResp) (int, string) {
	if args.Who == "gone" {
		return http.StatusNotFound, ""
	}
	if args.Who == "alex" {
		return http.StatusNotAcceptable, args.Who + " is not allowed!"
	}
	reply.Message = "Hey there, " + args.Who + "."
	reply.Who = args.Who
	return 0, ""
}

type HelloReqMulti struct {
	Names []string
}

type HelloRespMulti struct {
	Items []*HelloResp `json:"items"`
	Next  string       `json:"next,omitempty"`
	More  bool         `json:"more,omitempty"`
}

func SayHelloMulti(hello *HelloReqMulti, reply *HelloRespMulti) (int, string) {
	lenNames := len(hello.Names)
	if lenNames == 0 {
		return http.StatusNotFound, "There's nobody to say hello to!"
	}
	reply.Items = make([]*HelloResp, lenNames, lenNames)
	for i, name := range hello.Names {
		reply.Items[i] = &HelloResp{
			Message: fmt.Sprintf("Hello there, %s!", name),
			Who:     name,
		}
	}
	return 0, ""
}

type MultiplyArgs struct {
	A float64
	B float64
	K float32
}

type MultiplyResult struct {
	Result float64 `json:"result"`
	K      float32 `json:"k,omitempty"`
}

func Multiply(args *MultiplyArgs, result *MultiplyResult) (int, string) {
	result.Result = args.A * args.B
	result.K = args.K
	if args.K != 0 {
		result.Result *= float64(args.K)
	}
	return 0, ""
}

func init() {
	gotestApi := endpoints.NewApi("gotest", "v1")

	gotestApi.Method("POST", "/welcome", SayHelloMulti, "test.helloMulti")
	gotestApi.Method("GET", "/welcome/{who}", SayHello, "test.hello")
	gotestApi.Method("GET", "/multiply/{a}/{b}", Multiply, "test.multiply")

	endpoints.Handle("/api")
}
