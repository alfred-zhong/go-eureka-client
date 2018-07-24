# go-eureka-client

[![Build Status](https://www.travis-ci.org/alfred-zhong/go-eureka-client.svg?branch=master)](https://www.travis-ci.org/alfred-zhong/go-eureka-client) [![GoDoc](https://godoc.org/github.com/alfred-zhong/go-eureka-client?status.svg)](https://godoc.org/github.com/alfred-zhong/go-eureka-client) [![Go Report Card](https://goreportcard.com/badge/github.com/alfred-zhong/go-eureka-client)](https://goreportcard.com/report/github.com/alfred-zhong/go-eureka-client)

A simple package used for golang developing integrated with eureka.

This package wraps [hudl/fargo](https://github.com/hudl/fargo) to communicate with eureka server.

## Features now

* Cooperating with Spring Cloud.
* Instance register/deregister/heartbeat automatically.
* Simple http client.


## Basic Usage

### Register our instance

You should have a eureka server first. Usually you should setup it using Spring Cloud.

Now suppose that I setup a eureka server with url: `http://localhost:8761/eureka`.

Then my go code likes like:

```go
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
```

Now if you open url [http://localhost:8761/](http://localhost:8761/), you will see the instance.

After you kill the program(`SIGINT` and `SIGTERM`), the instance will deregister automatically. Or you can do it will `ins.Stop()` manually.


### Send request to app

```go
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
```

Run the code above. You should get it print `Hello World`.

You may also construct the **AppClient** by using `ins.GetAppClient("testapp")`, if you already have a **Instance**.


## Whatever

The package is not fully tested yet. If you run into any issues, please just raise it.

Issues and PRs are welcomed.
