# The missing Cloud Endpoints for Go

This package will let you write Cloud Endpoints backends in Go.
If you don't know what this is, see Google App Engine official implementations
for [Python][1] and [Java][2].

Start with `go get github.com/crhym3/go-endpoints/endpoints`.

Declare structs which describe your data. For instance:

```go
// Greeting is a datastore entity that represents a single greeting.
// It also serves as (a part of) a response of GreetingService.
type Greeting struct {
  Id      string    `json:"id,omitempty" datastore:"-"`
  Author  string    `json:"author"`
  Content string    `json:"content" datastore:",noindex"`
  Date    time.Time `json:"date"`
}

// GreetingsList is a response type of GreetingService.List method
type GreetingsList struct {
  Items []*Greeting `json:"items"`
}

// Request type for GreetingService.List
type GreetingsListReq struct {
  Limit int
}
```

Then, a service:

```go
// GreetingService can sign the guesbook, list all greetings and delete
// a greeting from the guestbook.
type GreetingService struct {
}

// List responds with a list of all greetings ordered by Date field.
// Most recent greets come first.
func (gs *GreetingService) List(
  r *http.Request, req *GreetingsListReq, resp *GreetingsList) error {

  ctx := appengine.NewContext(r)
  q := datastore.NewQuery("Greeting").Order("-Date").Limit(req.Limit)
  greets := make([]*Greeting, 0, req.Limit)
  keys, err := q.GetAll(ctx, &greets)
  if err != nil {
    return err
  }

  for i, k := range keys {
    greets[i].Id = k.Encode()
  }
  resp.Items = greets
  return nil
}
```

Last step is to make the above avaiable as an API and leverage all
the juicy stuff Cloud Endpoints are great at.

```go
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
```

Don't forget to add URL matching in app.yaml:

```yaml
application: my-app-id
version: v1
threadsafe: true

runtime: go
api_version: go1

handlers:
- url: /.*
  script: _go_app

# Important! Even though there's a catch all routing above, without
#these two line it's not going to work. Make sure you have this:
- url: /_ah/spi/.*
  script: _go_app
```

That's it. It is time to start the dev server and enjoy the discovery doc at
http://localhost:8080/_ah/api/discovery/v1/apis/greeting/v1/rest

Naturally, API Explorer works too:
http://localhost:8080/_ah/api/explorer

Time to deploy the app on appengine.appspot.com!

Samples
===

Check out the famous [TicTacToe app][3]. It has its own readme file.


Running tests
===

We currently use [aet tool][4] to simplify running tests on files that have
"appengine" or "appengine_internal" imports.

Check out the readme of that tool but, assuming you cloned this repo (so you can reach ./endpoints dir), initial setup process is actually pretty simple:

  - `go get github.com/crhym3/aegot/aet`
  - `aet init ./endpoints`

That's it. You should be able to run tests now with "aet test ./endpoints".


[1]: https://developers.google.com/appengine/docs/python/endpoints/
[2]: https://developers.google.com/appengine/docs/java/endpoints/
[3]: https://github.com/crhym3/go-endpoints/tree/master/tictactoeapp
[4]: https://github.com/crhym3/aegot
