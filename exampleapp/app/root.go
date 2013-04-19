package app

import (
	"fmt"
	"net/http"
)

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, welcomeMsg)
}

func init() {
	http.HandleFunc("/", handler)
}

const welcomeMsg = `
<html>
	<head><title>Go Endpoints</title></head>
	<body>
		<h1>Go Endpoints</h1>
		<p>Hello, gopher! Try this:</p>

		<ul>
			<li><a href="/_ah/api/discovery/v1/apis" target="_blank">
				/_ah/api/discovery/v1/apis
			</a></li>
			<li><a href="/_ah/api/discovery/v1/apis/greeting/v1/rest" target="_blank">
				/_ah/api/discovery/v1/apis/greeting/v1/rest
			</a></li>
		</ul>
		
		<iframe src="https://developers.google.com/apis-explorer/?base=http://localhost:8080/_ah/api#p/greeting/v1/" style="width: 100%; height: 600px; border: none;"></iframe>
		<!--<iframe src="https://developers.google.com/apis-explorer/?base=https://go-endpoints.appspot.com/_ah/api#p/greeting/v1/" style="width: 100%; height: 600px; border: none;"></iframe>-->

		<hr/>
		Source code:
		<a href="https://github.com/crhym3/go-endpoints" target="_blank">
			github.com/crhym3/go-endpoints
		</a>
	</body>
</html>
`
