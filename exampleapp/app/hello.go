package app

import (
	"fmt"
	"net/http"
)

func init() {
	http.HandleFunc("/", handler)
}

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, welcomeMsg)
}

const welcomeMsg = `
<html>
	<head><title>Go Endpoints</title></head>
	<body>
		<h1>Go Endpoints</h1>
		<p>Hello, gopher! Try this:</p>
		<ul>
			<li><a href="/api/discovery/v1/apis/discovery/v1/rest">
				/api/discovery/v1/apis/discovery/v1/rest
			</a></li>
			<li><a href="/api/discovery/v1/apis/gotest/v1/rest">
				/api/discovery/v1/apis/gotest/v1/rest
			</a></li>
			<li><a href="/api/gotest/v1/welcome/gopher">
				/api/gotest/v1/welcome/gopher
			</a></li>
			<li><a href="/api/gotest/v1/multiply/2/3">
				/api/gotest/v1/multiply/2/3
			</a></li>
			<li><a href="/api/gotest/v1/multiply/2/3?k=5">
				/api/gotest/v1/multiply/2/3?k=5
			</a></li>
			<li>TODO: /api/discovery/v1/apis</li>
			<li>TODO: /api/auth/v1/... OAuth2 (federated) server?</li>
		</ul>

		<p>Also, try POSTing:</p>
		<ul>
			<li><code>curl -X POST /api/gotest/v1/welcome</code></li>
			<li><code>curl -d '{"names": ["gopher", "alex", "dude"]}' /api/gotest/v1/welcome</code></li>
		</ul>

		<hr/>
		Source code:
		<a href="https://github.com/crhym3/go-endpoints" target="_blank">
			github.com/crhym3/go-endpoints
		</a>
	</body>
</html>
`
