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

// package hassi implements a writer that uploads selected data
// to the Home Assistant API.
//
// The package is configured as a section in the main config file
// under the '[hassi]' section, and the parameters are:
//  [hassi]
//  apikey=<apikey from Home Assistant>
//  url=<API endpoint>
//  # An entity entry for each tag to upload.
//  entity=<tag>,<Home Assistant entity ID>

package hassi

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	_ "net/url"
	"time"

	"github.com/aamcrae/MeterMan/db"
)

var hassiRate = flag.Int("hassirate", 1, "Home Assistant update rate (in minutes)")

type entity struct {
	tag string
	id  string
}

type hassi struct {
	d        *db.DB
	url      string
	key      string
	entities []entity
	client   *http.Client
}

func init() {
	db.RegisterInit(hassiInit)
}

func hassiInit(d *db.DB) error {
	sect := d.Config.GetSection("hassi")
	if sect == nil {
		return nil
	}
	key, err := sect.GetArg("apikey")
	if err != nil {
		return err
	}
	url, err := sect.GetArg("url")
	if err != nil {
		return err
	}
	h := &hassi{d: d, url: url, key: key, client: &http.Client{}}
	for _, e := range sect.Get("entity") {
		if len(e.Tokens) != 2 {
			return fmt.Errorf("hassiInit: bad entity config at %d", e.Lineno)
		}
		h.entities = append(h.entities, entity{e.Tokens[0], e.Tokens[1]})
		if d.Trace {
			log.Printf("hassi: tag %s: %s", e.Tokens[0], e.Tokens[1])
		}
	}
	intv := time.Minute*time.Duration(*hassiRate)
	d.AddUpdate(h, intv)
	log.Printf("Registered Home Assistant uploader (%s interval, %d entities)\n", intv, len(h.entities))
	return nil
}

// Update uploads any updated tags to Home Assistant.
func (h *hassi) Update(last time.Time, now time.Time) {
	for _, e := range h.entities {
		el := h.d.GetElement(e.tag)
		if isValid(el, last) {
			u := fmt.Sprintf("%s/%s", h.url, e.id)
			log.Printf("hassi: Sending element %s, value %f, url %s", e.tag, el.Get(), u)
		}
	}
}

// isValid will return true if the element is not nil and has been updated
// in the last interval.
func isValid(e db.Element, last time.Time) bool {
	return e != nil && !e.Timestamp().Before(last)
}
