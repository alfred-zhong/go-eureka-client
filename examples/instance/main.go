package main

import (
	"io"
	"net/http"
	"strings"

	"github.com/alfred-zhong/go-eureka-client"
)

func main() {
	// New instance with AppName "testapp" and port 9527.
	ins, err := eureka.NewInstanceWithPort("testapp", 9527, "http://localhost:8761/eureka")
	if err != nil {
		panic(err)
	}
	// Run instance. Note that this method will not block.
	if err := ins.Run(); err != nil {
		panic(err)
	}

	// Do anything you want. Maybe setup a http server.
	http.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		io.Copy(w, strings.NewReader("Hello World"))
	})
	if err := http.ListenAndServe(":9527", nil); err != nil {
		panic(err)
	}
}
