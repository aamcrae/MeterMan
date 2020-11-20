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

// package lcd implements a decoder that reads 7 segment display characters
// from an image.

package lcd

import (
	"fmt"
	"math/rand"
	"time"
)

// Defaults for bounding box margins.
const offMargin = 5
const onMargin = 2

// Segments, as enum and bit mask.
const (
	S_TL, M_TL = iota, 1 << iota // Top left
	S_TM, M_TM = iota, 1 << iota // Top middle
	S_TR, M_TR = iota, 1 << iota // Top right
	S_BR, M_BR = iota, 1 << iota // Bottom right
	S_BM, M_BM = iota, 1 << iota // Bottom middle
	S_BL, M_BL = iota, 1 << iota // Bottom left
	S_MM, M_MM = iota, 1 << iota // Middle
	SEGMENTS   = iota
)

// Base template for one type/size of 7-segment digit.
// Points are all relative to the top left corner position.
// When a digit is created using this template, the points are
// offset from the point where the digit is placed.
// The idea is that different size of digits use a different
// template, and that multiple digits are created from a single template.
type Template struct {
	name string            // Name of template
	line int               // Line width of segments
	bb   BBox              // Bounding box of digit
	off  PList             // List of points in off section
	dp   PList             // Decimal point offset (if any)
	mr   Point             // Middle right point
	ml   Point             // Middle right point
	tmr  Point             // Top middle right point
	tml  Point             // Top iddle left point
	bmr  Point             // Bottom middle right point
	bml  Point             // Bottom middle left point
	seg  [SEGMENTS]segment // Segments of digit
}

// Digit represents one 7-segment digit.
// It is typically created from a template, by offsetting the relative
// point values with the absolute point representing the top left of the digit.
// All point values are absolute as a result.
type Digit struct {
	index int // Digit index
	bb    BBox
	dp    PList
	tmr   Point
	tml   Point
	bmr   Point
	bml   Point
	off   PList
	seg   [SEGMENTS]segment
}

type segment struct {
	bb     BBox
	points PList
}

// LcdDecoder contains all the digit data required to decode
// the digits in an image.
type LcdDecoder struct {
	// Configuration values and flags.
	Threshold int  // Default on/off threshold
	History   int  // Size of moving average history
	MaxLevels int  // Maximum number of threshold levels
	Inverse   bool // True if darker is off e.g a LED rather than LCD.

	Digits    []*Digit             // List of digits to decode
	templates map[string]*Template // Templates used to create digits
	levelsMap map[int][]*levels    // Map of saved threshold levels keyed by quality (0-100)
	rng       *rand.Rand           // RNG
	curLevels *levels              // Current threshold levels

	// Current calibration levels summary
	Best        int // Current highest quality
	Worst       int // Current lowest quality
	LastQuality int // Last quality level
	LastGood    int // Last count of good scans
	LastBad     int // Last count of bad scans
	Count       int // Count of levels
	Total       int // Sum of all qualities
}

// Create a new LcdDecoder.
func NewLcdDecoder() *LcdDecoder {
	l := new(LcdDecoder)
	l.templates = make(map[string]*Template)
	// Init defaults.
	l.Threshold = 50  // Percentage threshold for on/off
	l.History = 5     // Size of moving average cache
	l.MaxLevels = 200 // Maximum size of threshold levels list
	l.levelsMap = make(map[int][]*levels)
	l.rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	return l
}

// Add a digit template.
// Each template describes the parameters of one type/size of digit.
// bb contains a list of 3 points representing the top right,
// bottom right and bottom left of the boundaries of the digit.
// These are signed offsets from the implied base of (0,0) representing
// the top left of the digit.
// dp is an optional point offset where a decimal place is located.
// width is the width of the segment in pixels.
// All point references in the template are relative to the origin of the digit.
func (l *LcdDecoder) AddTemplate(name string, bb []int, dp []int, width int) error {
	if _, ok := l.templates[name]; ok {
		return fmt.Errorf("Duplicate template entry: %s", name)
	}
	if len(bb) != 6 {
		return fmt.Errorf("Invalid bounding box length (expected 6, got %d)", len(bb))
	}
	if len(dp) != 0 && len(dp) != 2 {
		return fmt.Errorf("Invalid decimal point")
	}
	// Prepend the implied top left origin (0,0).
	bb = append([]int{0, 0}, bb...)
	t := &Template{name: name, line: width}
	for i := range t.bb {
		t.bb[i].X = bb[i*2]
		t.bb[i].Y = bb[i*2+1]
	}
	if len(dp) == 2 {
		t.dp = Point{X: dp[0], Y: dp[1]}.Block((width + 1) / 2)
	}
	// Initialise the bounding boxes representing the segments of the digit.
	// Calculate the middle points of the digit.
	t.mr = Split(t.bb[TR], t.bb[BR], 2)[0]
	t.tmr = Adjust(t.mr, t.bb[TR], width/2)
	t.bmr = Adjust(t.mr, t.bb[BR], width/2)
	t.ml = Split(t.bb[TL], t.bb[BL], 2)[0]
	t.tml = Adjust(t.ml, t.bb[TL], width/2)
	t.bml = Adjust(t.ml, t.bb[BL], width/2)
	// Build the 'off' point list using the middle blocks inside the
	// upper and lower squares of the segments.
	offbb1 := BBox{t.bb[TL], t.bb[TR], t.bmr, t.bml}.Inner(width + offMargin)
	offbb2 := BBox{t.tml, t.tmr, t.bb[BR], t.bb[BL]}.Inner(width + offMargin)
	t.off = offbb1.Points()
	t.off = append(t.off, offbb2.Points()...)
	// Create the bounding boxes of each segment in the digit.
	// The assignments must match the bit allocation in the lookup table.
	t.seg[S_TL].bb = SegmentBB(t.bb[TL], t.ml, t.bb[TR], t.mr, width, onMargin)
	t.seg[S_TM].bb = SegmentBB(t.bb[TL], t.bb[TR], t.bb[BL], t.bb[BR], width, onMargin)
	t.seg[S_TR].bb = SegmentBB(t.bb[TR], t.mr, t.bb[TL], t.ml, width, onMargin)
	t.seg[S_BR].bb = SegmentBB(t.mr, t.bb[BR], t.ml, t.bb[BL], width, onMargin)
	t.seg[S_BM].bb = SegmentBB(t.bb[BL], t.bb[BR], t.ml, t.mr, width, onMargin)
	t.seg[S_BL].bb = SegmentBB(t.ml, t.bb[BL], t.mr, t.bb[BR], width, onMargin)
	t.seg[S_MM].bb = SegmentBB(t.tml, t.tmr, t.bb[BL], t.bb[BR], width, onMargin)
	// For each segment, create a list of all the points within the segment.
	for i := range t.seg {
		t.seg[i].points = t.seg[i].bb.Points()
	}
	l.templates[name] = t
	return nil
}

// Add a digit using the named template. The template points are offset
// by the absolute point location of the digit (x, y).
func (l *LcdDecoder) AddDigit(name string, x, y int) (*Digit, error) {
	t, ok := l.templates[name]
	if !ok {
		return nil, fmt.Errorf("Unknown template %s", name)
	}
	index := len(l.Digits)
	d := &Digit{}
	d.index = index
	d.bb = t.bb.Offset(x, y)
	d.off = t.off.Offset(x, y)
	d.dp = t.dp.Offset(x, y)
	// Copy over the segment data from the template, offsetting the points
	// using the digit's origin.
	for i := 0; i < SEGMENTS; i++ {
		d.seg[i].bb = t.seg[i].bb.Offset(x, y)
		d.seg[i].points = t.seg[i].points.Offset(x, y)
	}
	d.dp = t.dp.Offset(x, y)
	d.tmr = t.tmr.Offset(x, y)
	d.tml = t.tml.Offset(x, y)
	d.bmr = t.bmr.Offset(x, y)
	d.bml = t.bml.Offset(x, y)
	l.Digits = append(l.Digits, d)
	return d, nil
}
