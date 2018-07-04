# Gublish
A publish / subscribe service in Go based on Server Sent Events

This is a test of multicast pub sub in Go, SSE is used for receiving data and POST requests for sending data - also useful for small projects. A simpler version of something like [Centrifugo](https://github.com/centrifugal/centrifugo).

Usage:
* Clone repo
* `go run *.go`
* Navigate to `localhost/v/anychannel` to subscribe to any channel (or `/s/anychannel` for the raw connection)
