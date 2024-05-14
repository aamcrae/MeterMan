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
	"strconv"
	"strings"
)

// ConfigOrDefault will return a value if it is
// non-zero (or empty), or a default value
func ConfigOrDefault[T comparable](conf, def T) T {
	// Check for zero value
	if conf == *new(T) {
		return def
	}
	return conf
}

// FmtFloat is a custom float formatter that
// has a fixed precision of 2 decimal places with trailing zeros removed.
func FmtFloat(f float64) string {
	s := strconv.FormatFloat(f, 'f', 2, 64)
	s = strings.TrimRight(s, "0")
	last := len(s) - 1
	if s[last] == '.' {
		s = s[:last]
	}
	return s
}
