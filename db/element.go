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

// Element represents a data item that is updated by the readers.
type Element interface {
	Update(v float64)   // Update element with new value.
	Midnight()          // Called when it is midnight for end-of-day processing
	Get() float64       // Get the element's value
	Updated() bool      // Return true if value has been updated in this interval.
	ClearUpdate()       // Reset the update flag.
	Checkpoint() string // Return a checkpoint string.
}

// Acc is a common interface for accumulators.
type Acc interface {
	Element
	Daily() float64 // Return the daily total.
}
