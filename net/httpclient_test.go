package net

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/zalando/skipper/tracing/tracers/basic"
)

func TestClient(t *testing.T) {
	tracer, err := basic.InitTracer([]string{"recorder=in-memory"})
	if err != nil {
		t.Fatalf("Failed to get a tracer: %v", err)
	}

	for _, tt := range []struct {
		name      string
		options   Options
		tokenFile string
		wantErr   bool
	}{
		{
			name:    "All defaults, with request should have a response",
			wantErr: false,
		},
		{
			name: "With tracer",
			options: Options{
				Tracer: tracer,
			},
			wantErr: false,
		},
		{
			name: "With tracer and span name",
			options: Options{
				Tracer:                  tracer,
				OpentracingComponentTag: "mytag",
				OpentracingSpanName:     "foo",
			},
			wantErr: false,
		},
		{
			name:      "With token",
			options:   Options{},
			tokenFile: "token",
			wantErr:   false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tok := "mytoken1"

			s := startTestServer(func(r *http.Request) {
				if tt.options.OpentracingSpanName != "" && tt.options.Tracer != nil {
					if r.Header.Get("Ot-Tracer-Sampled") == "" ||
						r.Header.Get("Ot-Tracer-Traceid") == "" ||
						r.Header.Get("Ot-Tracer-Spanid") == "" {
						t.Errorf("One of the OT Tracer headers are missing: %v", r.Header)
					}
				}

				if tt.tokenFile != "" {
					switch auth := r.Header.Get("Authorization"); auth {
					case "Bearer " + tok:
						// success
					default:
						t.Errorf("Wrong Authorization header '%s'", auth)
					}
				}
			})
			defer s.Close()

			if tt.tokenFile != "" {
				dir, err := ioutil.TempDir("/tmp", "skipper-httpclient-test")
				if err != nil {
					t.Fatalf("Failed to create temp dir: %v", err)
				}
				defer os.RemoveAll(dir) // clean up
				tokenFile := filepath.Join(dir, tt.tokenFile)
				if err := ioutil.WriteFile(tokenFile, []byte(tok), 0600); err != nil {
					t.Fatalf("Failed to create token file: %v", err)
				}
				tt.options.BearerTokenFile = tokenFile
			}

			cli := NewClient(tt.options)
			defer cli.Close()

			u := "http://" + s.Listener.Addr().String() + "/"

			_, err = cli.Get(u)
			if (err != nil) != tt.wantErr {
				t.Errorf("Failed to do GET request error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestTransport(t *testing.T) {
	tracer, err := basic.InitTracer(nil)
	if err != nil {
		t.Fatalf("Failed to get a tracer: %v", err)
	}

	for _, tt := range []struct {
		name        string
		options     Options
		spanName    string
		bearerToken string
		req         *http.Request
		wantErr     bool
	}{
		{
			name:    "All defaults, with request should have a response",
			req:     httptest.NewRequest("GET", "http://example.com/", nil),
			wantErr: false,
		},
		{
			name: "With opentracing, should have opentracing headers",
			options: Options{
				Tracer: tracer,
			},
			spanName: "myspan",
			req:      httptest.NewRequest("GET", "http://example.com/", nil),
			wantErr:  false,
		},
		{
			name:        "With bearer token request should have a token in the request observed by the endpoint",
			bearerToken: "my-token",
			req:         httptest.NewRequest("GET", "http://example.com/", nil),
			wantErr:     false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := startTestServer(func(r *http.Request) {
				if r.Method != tt.req.Method {
					t.Errorf("wrong request method got: %s, want: %s", r.Method, tt.req.Method)
				}

				if tt.wantErr {
					return
				}

				if tt.spanName != "" && tt.options.Tracer != nil {
					if r.Header.Get("Ot-Tracer-Sampled") == "" ||
						r.Header.Get("Ot-Tracer-Traceid") == "" ||
						r.Header.Get("Ot-Tracer-Spanid") == "" {
						t.Errorf("One of the OT Tracer headers are missing: %v", r.Header)
					}
				}

				if tt.bearerToken != "" {
					if r.Header.Get("Authorization") != "Bearer my-token" {
						t.Errorf("Failed to have a token, but want to have it, got: %v, want: %v", r.Header.Get("Authorization"), "Bearer "+tt.bearerToken)
					}
				}
			})

			defer s.Close()

			rt := NewTransport(tt.options)
			defer rt.Close()

			if tt.spanName != "" {
				rt = WithSpanName(rt, tt.spanName)
			}
			if tt.bearerToken != "" {
				rt = WithBearerToken(rt, tt.bearerToken)
			}

			if tt.req != nil {
				tt.req.URL.Host = s.Listener.Addr().String()
			}
			_, err := rt.RoundTrip(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("Transport.RoundTrip() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

		})
	}
}

type requestCheck func(*http.Request)

func startTestServer(check requestCheck) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		check(r)

		w.Header().Set("X-Test-Response-Header", "response header value")
		w.WriteHeader(http.StatusOK)
	}))
}
