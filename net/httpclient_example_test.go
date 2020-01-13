package net_test

import (
	"log"
	"net/http"
	"time"

	"github.com/lightstep/lightstep-tracer-go"
	"github.com/zalando/skipper/net"
)

func ExampleTransport() {
	tracer := lightstep.NewTracer(lightstep.Options{})

	cli := net.NewTransport(net.Options{
		Tracer: tracer,
	})
	defer cli.Close()
	cli = net.WithSpanName(cli, "myspan")
	cli = net.WithBearerToken(cli, "mytoken")

	u := "http://127.0.0.1:12345/foo"
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}

	rsp, err := cli.RoundTrip(req)
	if err != nil {
		log.Fatalf("Failed to do request: %v", err)
	}
	log.Printf("rsp code: %v", rsp.StatusCode)
}

func ExampleClient() {
	tracer := lightstep.NewTracer(lightstep.Options{})

	cli := net.NewClient(net.Options{
		Tracer:                     tracer,
		OpentracingComponentTag:    "testclient",
		OpentracingSpanName:        "clientSpan",
		BearerTokenRefreshInterval: 10 * time.Second,
		BearerTokenFile:            "/tmp/foo.token",
		IdleConnTimeout:            2 * time.Second,
	})
	defer func() {
		println("closing...")
		cli.Close()
		println("stopped")
	}()

	u := "http://127.0.0.1:12345/foo"

	for i := 0; i < 15; i++ {
		rsp, err := cli.Get(u)
		if err != nil {
			log.Fatalf("Failed to do request: %v", err)
		}
		log.Printf("rsp code: %v", rsp.StatusCode)
		time.Sleep(1 * time.Second)
	}
}
