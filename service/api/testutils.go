package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"testing"
)

// freePort gets a free port. Courtesy of
// https://gist.github.com/sevkin/96bdae9274465b2d09191384f86ef39d
func freePort() (port int, err error) {
	var a *net.TCPAddr
	if a, err = net.ResolveTCPAddr("tcp", "localhost:0"); err == nil {
		var l *net.TCPListener
		if l, err = net.ListenTCP("tcp", a); err == nil {
			defer l.Close()
			return l.Addr().(*net.TCPAddr).Port, nil
		}
	}
	return
}

// freeLocalNoFail is a wrapper around freePort(), where the returner err
// raises a t.Fatal if not nil, using the given testing.T. Otherwise, the
// port will be returned as a str in the format ":x".
func freeLocalNoFail(t *testing.T) string {
	port, err := freePort()
	if err != nil {
		t.Fatal("could not get a free port;", err)
	}

	return fmt.Sprintf(":%d", port)
}

// post is a convenience func on top of http.Post, which provides simple-to-use
// generic syntax. It simply tries to encode "data" into a json, post it to the
// url, then attempts to unpack the response from a json into an instance of T
// which is to be returned. The error is not nil on these conditions:
// - "data" cannot be encoded into a json.
// - http.Post(...) returns an error.
// - T cannot be decoded from a json.
func post[T any](url string, data any) (T, error) {
	var r T

	// Encode send data.
	b, err := json.Marshal(data)
	if err != nil {
		return r, err
	}
	// Post and get reply.
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(b))
	if err != nil {
		return r, err
	}
	defer resp.Body.Close()

	// Decode end return data.
	b, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return r, err
	}

	return r, json.Unmarshal(b, &r)
}
