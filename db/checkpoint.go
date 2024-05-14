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
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"
)

// readCheckpoint reads the checkpoint data into a map.
// The checkpoint file contains lines of the form:
//
//	<tag>:<checkpoint string>
//
// When a new element is created, the tag is used to find the checkpoint string
// to be passed to the element's init function so that the element's value can be restored.
func (d *DB) readCheckpoint(file string) error {
	f, err := os.Open(file)
	if err != nil {
		// If the checkpoint file doesn't exist, skip trying to read it.
		log.Printf("Unable to read %s (%v), no checkpoint data", file, err)
		return nil
	}
	defer f.Close()
	log.Printf("Reading checkpoint data from %s", file)
	r := bufio.NewReader(f)
	lineno := 0
	for {
		lineno++
		s, err := r.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				return fmt.Errorf("checkpoint read %s: line %d: %v", file, lineno, err)
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

// writeCheckpoint saves the values of the elements in the database to a checkpoint file.
func (d *DB) writeCheckpoint(file string, now time.Time) {
	if d.Trace {
		log.Printf("Writing checkpoint data to %s", file)
	}
	f, err := os.Create(file)
	if err != nil {
		log.Printf("Checkpoint file create: %s %v", file, err)
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
