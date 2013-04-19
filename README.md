# Go Endpoints

This is a very pre-alpha version of Cloud Endpoints for Go, though it works on
appspot too, check it out!

[go-endpoints.appspot.com](https://go-endpoints.appspot.com)

## Getting Started
If you really want to try it out:

`go get github.com/crhym3/go-endpoints/endpoints`

or, if you prefer Google Code:

`go get code.google.com/p/go-endpoints/endpoints`

Then copy [github.com/crhym3/go-endpoints/tree/master/exampleapp](https://github.com/crhym3/go-endpoints/tree/master/exampleapp) and run dev server:

`GOSDK/dev_appserver.py exampleapp/`

It's a simple guestbook app taken from https://developers.google.com/appengine/docs/go/gettingstarted/usingdatastore

***

Browse to [http://localhost:8080](http://localhost:8080) and you should be able to see something like
this:

![http://localhost:8080](https://lh6.googleusercontent.com/-9wk96-rUcvo/UXFeTpzWi8I/AAAAAAAART0/4D31sBNdppk/s900/Screen+Shot+2013-04-19+at+5.06.12+PM.png)

***

Try signing the guestbook:

![Sign guesbook](https://lh5.googleusercontent.com/-ravF58cJy9Q/UXFeR8aC5aI/AAAAAAAARTc/ZCDqI8vpzq4/s826/Screen+Shot+2013-04-19+at+5.07.46+PM.png)

![Sign response](https://lh4.googleusercontent.com/-b_R1g2iYTDU/UXFeREDzSuI/AAAAAAAARTU/rwM47Sp-xf8/s571/Screen+Shot+2013-04-19+at+5.08.06+PM.png)

***

List greetings:

![List greetings](https://lh3.googleusercontent.com/-lMd8bB2zjmw/UXFeQeqsM9I/AAAAAAAARTM/oqhCYNxmeWc/s574/Screen+Shot+2013-04-19+at+5.08.30+PM.png)

## Things I'm not happy about and will be working on (or anyone else's welcome!)

  - Add tests
  - Remove spaghetti code
  - A more flexible service method signature
    (*http.Request is not always needed, and appengine.Context would be enough)
  - A nicer solution for setting all kinds of parameters of discovery doc.
