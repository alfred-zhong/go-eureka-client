package main

import (
	"fmt"
	"io/ioutil"

	"github.com/alfred-zhong/go-eureka-client"
)

func main() {
	// New a AppClient with AppName "testapp".
	c, err := eureka.NewAppClient("testapp", "http://localhost:8761/eureka")
	if err != nil {
		panic(err)
	}

	// Send GET request to the app with path "hello"
	res, err := c.Get("/hello", nil)
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()

	bs, err := ioutil.ReadAll(res.Body)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(bs))
}
