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

package main

import (
	"flag"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"log"
	"net/http"
	"os"

	"github.com/aamcrae/MeterMan/lcd"
)

var port = flag.Int("port", 8100, "Port for image server")
var refresh = flag.Int("refresh", 4, "Number of seconds before image refresh")

type server struct {
	l   *lcd.LcdDecoder
	img image.Image
	str string
}

// Initialise a http server.
func serverInit() (*server, error) {
	h, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("hostname error: %v", err)
	}
	mux := http.NewServeMux()
	s := &server{}
	mux.HandleFunc("/filled.jpg", func(w http.ResponseWriter, req *http.Request) {
		s.sendImage(w, true)
	})
	mux.HandleFunc("/outline.jpg", func(w http.ResponseWriter, req *http.Request) {
		s.sendImage(w, false)
	})
	mux.HandleFunc("/outline.html", func(w http.ResponseWriter, req *http.Request) {
		s.page(w, false)
	})
	mux.HandleFunc("/filled.html", func(w http.ResponseWriter, req *http.Request) {
		s.page(w, true)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/" {
			http.NotFound(w, req)
			return
		}
		s.page(w, false)
	})
	go func() {
		log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), mux))
	}()
	log.Printf("Server on http:%s:%d/outline.html\n", h, *port)
	return s, nil
}

// Update image
func (s *server) updateImage(img image.Image, str string) {
	s.img = img
	s.str = str
}

// Update decoder
func (s *server) updateDecoder(l *lcd.LcdDecoder) {
	s.l = l
}

func (s *server) page(w http.ResponseWriter, filled bool) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, "<html><head><meta http-equiv=\"refresh\" content=\"%d\"></head><body>", *refresh)
	if len(s.str) != 0 {
		fmt.Fprintf(w, "Decoded segments = %s<br>", s.str)
	}
	if filled {
		fmt.Fprintf(w, "<a href=\"outline.html\">Outline image</a>")
		fmt.Fprintf(w, "<p><img src=\"filled.jpg\">")
	} else {
		fmt.Fprintf(w, "<a href=\"filled.html\">Filled image</a>")
		fmt.Fprintf(w, "<p><img src=\"outline.jpg\">")
	}
	fmt.Fprintf(w, "</body>")
}

func (s *server) sendImage(w http.ResponseWriter, fill bool) {
	img := s.img
	if s.l == nil || img == nil {
		http.Error(w, "No image yet", http.StatusInternalServerError)
		return
	}
	// Copy the image first.
	b := img.Bounds()
	dst := image.NewRGBA(b)
	draw.Draw(dst, b, img, b.Min, draw.Src)
	s.l.MarkSamples(dst, fill)
	w.Header().Set("Content-Type", "image/jpeg")
	err := jpeg.Encode(w, dst, nil)
	if err != nil {
		http.Error(w, "JPEG encode error", http.StatusInternalServerError)
	}
}
