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
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/aamcrae/MeterMan/db"
)

var port = flag.Int("port", 0, "Port for API server")

type apiServer struct {
	d *db.DB
}

type Item struct {
	Value     int   `json: "value"`
	Timestamp int64 `json: "timestamp"`
}

type Data struct {
	Power       Item `json: "power"`
	Import      Item `json: "import"`
	Export      Item `json: "export"`
	Generated   Item `json: "generated"`
	Consumption int  `json: "consumption"`
	Available   int  `json: "available"`
}

func init() {
	db.RegisterInit(serverInit)
}

// Initialise a http server.
func serverInit(d *db.DB) error {
	if *port == 0 {
		return nil
	}
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

// Handler for API requests.
func (s *apiServer) api(w http.ResponseWriter, req *http.Request) {
	if s.d.Trace {
		log.Printf("Request: %s", req.URL.String())
	}
	var c Data
	s.gauge(&c.Power, db.G_TP, 1000)
	s.daily(&c.Import, db.A_IMPORT, 1000)
	s.daily(&c.Export, db.A_EXPORT, 1000)
	s.daily(&c.Generated, db.A_GEN_TOTAL, 1000)
	c.Consumption = c.Generated.Value + c.Import.Value - c.Export.Value
	if c.Power.Value < 0 {
		c.Available = -c.Power.Value
	}
	m, err := json.Marshal(c)
	if err != nil {
		log.Printf("api: marshal: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(m)
}

// Fill in item from the gauge
func (s *apiServer) gauge(i *Item, n string, scale float64) {
	e := s.d.GetElement(n)
	if e == nil {
		return
	}
	i.Value = int(e.Get() * scale)
	i.Timestamp = e.Timestamp().Unix()
}

// Fill in item from the daily value of the accumlator
func (s *apiServer) daily(i *Item, n string, scale float64) {
	e := s.d.GetAccum(n)
	if e == nil {
		return
	}
	i.Value = int(e.Daily() * scale)
	i.Timestamp = e.Timestamp().Unix()
}

// status provides a HTML status page.
func (s *apiServer) status(w http.ResponseWriter, req *http.Request) {
	if s.d.Trace {
		log.Printf("Request: %s", req.URL.String())
	}
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, "<table border=\"1\"><tr><th>Tag</th><th>Value</th><th>Daily</th><th>Timestamp</th></tr>")
	m := s.d.GetElements()
	// Sort in key order.
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := m[k]
		fmt.Fprintf(w, "<tr><td><bold>%s</bold></td>", k)
		fmt.Fprintf(w, "<td style=\"text-align:right\">%f</td>", v.Get())
		switch vt := v.(type) {
		case db.Acc:
			fmt.Fprintf(w, "<td style=\"text-align:right\">%f</td>", vt.Daily())
		default:
			fmt.Fprintf(w, "<td> </td>")
		}
		fmt.Fprintf(w, "<td>%s</td></tr>", v.Timestamp().Format(time.UnixDate))
	}
	fmt.Fprintf(w, "</table>")
}
