/*
This package will let you write Cloud Endpoints backend in Go.

Usage

Declare structs which describe your data. For instance:

	// Greeting is a datastore entity that represents a single greeting.
	// It also serves as (a part of) a response of GreetingService.
	type Greeting struct {
	  Key     *datastore.Key `json:"id" datastore:"-"`
	  Author  string         `json:"author"`
	  Content string         `json:"content" datastore:",noindex" endpoints:"req"`
	  Date    time.Time      `json:"date"`
	}

	// GreetingsList is a response type of GreetingService.List method
	type GreetingsList struct {
	  Items []*Greeting `json:"items"`
	}

	// Request type for GreetingService.List
	type GreetingsListReq struct {
	  Limit int `json:"limit" endpoints:"d=10"`
	}


Then, a service:

	// GreetingService can sign the guesbook, list all greetings and delete
	// a greeting from the guestbook.
	type GreetingService struct {
	}

	// List responds with a list of all greetings ordered by Date field.
	// Most recent greets come first.
	func (gs *GreetingService) List(
	  r *http.Request, req *GreetingsListReq, resp *GreetingsList) error {

	  if req.Limit <= 0 {
	    req.Limit = 10
	  }

	  c := endpoints.NewContext(r)
	  q := datastore.NewQuery("Greeting").Order("-Date").Limit(req.Limit)
	  greets := make([]*Greeting, 0, req.Limit)
	  keys, err := q.GetAll(c, &greets)
	  if err != nil {
	    return err
	  }

	  for i, k := range keys {
	    greets[i].Key = k
	  }
	  resp.Items = greets
	  return nil
	}


Last step is to make the above available as a discoverable API
and leverage all the juicy stuff Cloud Endpoints are great at.

	import "github.com/crhym3/go-endpoints/endpoints"

	func init() {
	  greetService := &GreetingService{}
	  api, err := endpoints.RegisterService(greetService,
	    "greeting", "v1", "Greetings API", true)
	  if err != nil {
	    panic(err.Error())
	  }

	  info := api.MethodByName("List").Info()
	  info.Name, info.HttpMethod, info.Path, info.Desc =
	    "greets.list", "GET", "greetings", "List most recent greetings."

	  endpoints.HandleHttp()
	}


Don't forget to add URL matching in app.yaml:

	application: my-app-id
	version: v1
	threadsafe: true

	runtime: go
	api_version: go1

	handlers:
	- url: /.*
	  script: _go_app

	# Important! Even though there's a catch all routing above,
	# without these two lines it's not going to work.
	# Make sure you have this:
	- url: /_ah/spi/.*
	  script: _go_app

That's it. It is time to start dev server and enjoy the discovery doc:
http://localhost:8080/_ah/api/explorer


Custom types

You can define your own types and use them directly as a field type in a
service method request/response as long as they implement json.Marshaler and
json.Unmarshaler interfaces.

Let's say we have this method:

	func (s *MyService) ListItems(r *http.Request, req *ListReq, resp *ItemsList) error {
	  // fetch a list of items
	}

where ListReq and ItemsList are defined as follows:

	type ListReq struct {
	    Limit  int        `json:"limit,string" endpoints:"d=10,max=100"`
	    Page *QueryMarker `json:"cursor"`
	}

	type ItemsList struct {
	    Items []*Item      `json:"items"`
	    Next  *QueryMarker `json:"next,omitempty"`
	}

What's interesting here is ListReq.Page and ItemsList.Next fields which are
of type QueryMarker:

	import "appengine/datastore"

	type QueryMarker struct {
	    datastore.Cursor
	}

	func (qm *QueryMarker) MarshalJSON() ([]byte, error) {
	    return []byte(`"` + qm.String() + `"`), nil
	}

	func (qm *QueryMarker) UnmarshalJSON(buf []byte) error {
	    if len(buf) < 2 || buf[0] != '"' || buf[len(buf)-1] != '"' {
	        return errors.New("QueryMarker: bad cursor value")
	    }
	    cursor, err := datastore.DecodeCursor(string(buf[1 : len(buf)-1]))
	    if err != nil {
	        return err
	    }
	    *qm = QueryMarker{cursor}
	    return nil
	}

Now that our QueryMarker implements required interfaces we can use ListReq.Page
field as if it were a `datastore.Cursor` in our service method, for instance:

	func (s *MyService) ListItems(r *http.Request, req *ListReq, list *ItemsList) error {
	    c := endpoints.NewContext(r)
	    list.Items = make([]*Item, 0, req.Limit)

	    q := datastore.NewQuery("Item").Limit(req.Limit)
	    if req.Page != nil {
	        q = q.Start(req.Page.Cursor)
	    }

	    var iter *datastore.Iterator
	    for iter := q.Run(c); ; {
	        var item Item
	        key, err := iter.Next(&item)
	        if err == datastore.Done {
	            break
	        }
	        if err != nil {
	          return err
	        }
	        item.Key = key
	        list.Items = append(list.Items, &item)
	    }

	    cur, err := iter.Cursor()
	    if err != nil {
	        return err
	    }
	    list.Next = &QueryMarker{cur}
	    return nil
	}

A serialized ItemsList would then look something like this:

	{
	  "items": [
	    {
	      "id": "5629499534213120",
	      "name": "A TV set",
	      "price": 123.45
	    }
	  ],
	  "next": "E-ABAIICImoNZGV2fmdvcGhtYXJrc3IRCxIEVXNlchiAgICAgICACgwU"
	}

Another nice thing about this is, some types in appengine/datastore package
already implement json.Marshal and json.Unmarshal.

Take, for instance, datastore.Key. I could use it as an ID in my JSON response
out of the box, if I wanted to:

	type User struct {
	    Key *datastore.Key `json:"id" datastore:"-"`
	    Name string        `json:"name" datastore:"name"`
	    Role string        `json:"role" datastore:"role"`
	    Email string       `json:"email" datastore:"email"`
	}

	type GetUserReq struct {
	    Key *datastore.Key `json:"id"`
	}

	// defined with "users/{id}" path template
	func (s *MyService) GetUser(r *http.Request, req *GetUserReq, user *User) error {
	  c := endpoints.NewContext(r)
	  if err := datastore.Get(c, req.Key, user); err != nil {
	    return err
	  }
	  user.Key = req.Key
	  return nil
	}

JSON would then look something like this:

	GET /_ah/api/myapi/v1/users/ag1kZXZ-Z29waG1hcmtzchELEgRVc2VyGICAgICAgIAKDA

	{
	  "id": "ag1kZXZ-Z29waG1hcmtzchELEgRVc2VyGICAgICAgIAKDA",
	  "name": "John Doe",
	  "role": "member",
	  "email": "user@example.org"
	}


Field tags

Go Endpoints has its own field tag "endpoints" which you can use to let your
clients know what a service method data constraints are (on input):

	- req, means "required".
	- d, default value, cannot be used together with req.
	- min and max constraints. Can be used only on int and uint (8/16/32/64 bits).
	- desc, a field description. Cannot contain a "," (comma) for now.

Let's see an example:

	type TaggedStruct struct {
	    A int    `endpoints:"req,min=0,max=100,desc=An int field"`
	    B int    `endpoints:"d=10,min=1,max=200"`
	    C string `endpoints:"req,d=Hello gopher,desc=A string field"`
	}

	- A field is required and has min & max constrains, is described as "An int field"
	- B field is not required, defaults to 10 and has min & max constrains
	- C field is required, defaults to "Hello gopher", is described as "A string field"

JSON tag and path templates

You can use JSON tags to shape your service method's response (the output).

Endpoints will honor Go's encoding/json marshaling rules
(http://golang.org/pkg/encoding/json/#Marshal), which means having this struct:

	type TaggedStruct struct {
	    A       int
	    B       int    `json:"myB"`
	    C       string `json:"c"`
	    Skipped int    `json:"-"`
	}

a service method path template could then look like:

	some/path/{A}/other/{c}/{myB}

Notice, the names are case-sensitive.

Naturally, you can combine json and endpoints tags to use a struct for both
input and output:

	type TaggedStruct struct {
	    A       int    `endpoints:"req,min=0,max=100,desc=An int field"`
	    B       int    `json:"myB" endpoints:"d=10,min=1,max=200"`
	    C       string `json:"c" endpoints:"req,d=Hello gopher,desc=A string field"`
	    Skipped int    `json:"-"`
	}

Long integers (int64, uint64)

As per Type and Format Summary (https://developers.google.com/discovery/v1/type-format):

	a 64-bit integer cannot be represented in JSON (since JavaScript and JSON
	support integers up to 2^53). Therefore, a 64-bit integer must be
	represented as a string in JSON requests/responses

In this case, it is sufficient to append ",string" to the json tag:

	type Int64Struct struct {
	  Id int64 `json:",string"`
	}


Generate client libraries

Once an app is deployed on appspot.com, we can use the discovery doc to generate
libraries for different clients.

Android

	$ URL='https://my-app-id.appspot.com/_ah/api/discovery/v1/apis/greeting/v1/rest'
	$ curl -s $URL > greetings.rest.discovery

	# Optionally check the discovery doc
	$ less greetings.rest.discovery

	$ GO_SDK/endpointscfg.py gen_client_lib java greetings.rest.discovery

You should be able to find ./greetings.rest.zip file with Java client source
code and its dependencies.

Once you have that, follow the official guide
https://developers.google.com/appengine/docs/python/endpoints/consume_android.

iOS

	# Note the rpc suffix in the URL:
	$ URL='https://my-app-id.appspot.com/_ah/api/discovery/v1/apis/greeting/v1/rpc'
	$ curl -s $URL > greetings.rpc.discovery

	# optionally check the discovery doc
	$ less greetings.rpc.discovery

Then, feed greetings.rpc.discovery file to the library generator on OS X
as described in the official guide:
https://developers.google.com/appengine/docs/python/endpoints/consume_ios

JavaScript

There's really nothing to generate for JavaScript, you just use it!

Here's the official guide:
https://developers.google.com/appengine/docs/python/endpoints/consume_js


Other docs

Wiki pages on the github repo:
https://github.com/crhym3/go-endpoints/wiki


Samples

Check out TicTacToe sample:
https://github.com/crhym3/go-tictactoe

Or play it on the live demo app at https://go-endpoints.appspot.com/tictactoe


Running tests

We currently use aet tool (https://github.com/crhym3/aegot) to simplify running
tests on files that have "appengine" or "appengine_internal" imports.

Check out the readme of that tool but, assuming you cloned this repo
(so you can reach ./endpoints dir), the initial setup process is pretty simple:

  - go get github.com/crhym3/aegot/aet
  - aet init ./endpoints

That's it. You should be able to run tests with "aet test ./endpoints" now.
*/
package endpoints
