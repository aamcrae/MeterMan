// Copyright 2019 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// package server implements a HTTP API server and status server.

package server

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/aamcrae/MeterMan/db"
)

var port = flag.Int("port", 8080, "Port for API server")

type apiServer struct {
	d *db.DB
}

func init() {
	db.RegisterInit(serverInit)
}

// Initialise a http server.
func serverInit(d *db.DB) error {
	mux := http.NewServeMux()
	s := &apiServer{d: d}
	mux.HandleFunc("/api", func(w http.ResponseWriter, req *http.Request) {
		s.api(w, req)
	})
	mux.HandleFunc("/status", func(w http.ResponseWriter, req *http.Request) {
		s.status(w, req)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/" {
			http.NotFound(w, req)
			return
		}
		s.status(w, req)
	})
	go func() {
		log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), mux))
	}()
	log.Printf("Registered HTTP API and status server on port %d\n", *port)
	return nil
}

func (s *apiServer) api(w http.ResponseWriter, req *http.Request) {
	if s.d.Trace {
		log.Printf("Request: %s", req.URL.String())
	}
	fmt.Fprintf(w, "Welcome to the API server page!")
}

// status provides a HTML status page.
func (s *apiServer) status(w http.ResponseWriter, req *http.Request) {
	if s.d.Trace {
		log.Printf("Request: %s", req.URL.String())
	}
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, "<h2>Current values of all the elements are:</h2>")
	fmt.Fprintf(w, "<table><tr><th>Tag</th><th>Value</th><th>Timestamp</th></tr>")
	for k, v := range s.d.GetElements() {
		fmt.Fprintf(w, "<tr><td><bold>%s</bold></td><td>%f</td><td>%s</td></tr>", k, v.Get(), v.Timestamp().Format(time.UnixDate))
	}
	fmt.Fprintf(w, "</table>")
}
