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

package db

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"
)

var checkpointTick = flag.Int("checkpointrate", 1, "Checkpoint interval (in minutes)")
var checkpoint = flag.String("checkpoint", "", "Checkpoint file")

// writeCheckpoint saves the values of the elements in the database to a checkpoint file.
func (d *DB) writeCheckpoint(now time.Time) {
	if len(*checkpoint) == 0 {
		return
	}
	if d.Trace {
		log.Printf("Writing checkpoint data to %s\n", *checkpoint)
	}
	f, err := os.Create(*checkpoint)
	if err != nil {
		log.Printf("Checkpoint file create: %s %v\n", *checkpoint, err)
		return
	}
	defer f.Close()
	wr := bufio.NewWriter(f)
	defer wr.Flush()
	for n, e := range d.elements {
		s := e.Checkpoint()
		if len(s) != 0 {
			fmt.Fprintf(wr, "%s:%s\n", n, s)
		}
	}
	fmt.Fprintf(wr, "%s:%d\n", C_TIME, now.Unix())
}

// Checkpoint reads the checkpoint data into a map.
// The checkpoint file contains lines of the form:
//
//    <tag>:<checkpoint string>
//
// When a new element is created, the tag is used to find the checkpoint string
// to be passed to the element's init function so that the element's value can be restored.
func (d *DB) readCheckpoint() error {
	if len(*checkpoint) == 0 {
		return nil
	}
	// Add a callback to checkpoint the database at the specified interval.
	d.AddCallback(time.Minute*time.Duration(*checkpointTick), func(last time.Time, now time.Time) {
		d.writeCheckpoint(now)
	})
	log.Printf("Reading checkpoint data from %s\n", *checkpoint)
	f, err := os.Open(*checkpoint)
	if err != nil {
		return fmt.Errorf("checkpoint file %s: %v", *checkpoint, err)
	}
	defer f.Close()
	r := bufio.NewReader(f)
	lineno := 0
	for {
		lineno++
		s, err := r.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				return fmt.Errorf("checkpoint read %s: line %d: %v", *checkpoint, lineno, err)
			}
			return nil
		}
		s = strings.TrimSuffix(s, "\n")
		i := strings.IndexRune(s, ':')
		if i > 0 {
			d.checkpoint[s[:i]] = s[i+1:]
			if d.Trace {
				log.Printf("Checkpoint entry %s = %s\n", s[:i], s[i+1:])
			}
		}
	}
}
