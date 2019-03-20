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

// package sma implements reading telemetry data from a SMA solar inverter.

package sma

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"time"

	"github.com/aamcrae/MeterMan/core"
	"github.com/aamcrae/config"
)

var smatimeout = flag.Int("inverter_timeout", 10, "Inverter timeout in seconds")
var smapoll = flag.Int("inverter-poll", 120, "Inverter poll time (seconds)")
var trace = flag.Bool("trace", false, "Enable trace packet dumps")

const maxPacketSize = 8 * 1024

var packet_header = []byte{'S', 'M', 'A', 0, 0, 0x04, 0x02, 0xA0,
	0, 0, 0, 0x01, 0, 0}

const signature = uint32(0x65601000)
const nan64 = 0x8000000000000000
const nan32 = 0x80000000
const nanu64 = 0xFFFFFFFFFFFFFFFF
const nanu32 = 0xFFFFFFFF

const password_length = 12
const password_enc = 0x88

const (
	DT_ULONG  = 0
	DT_STATUS = 8
	DT_STRING = 16
	DT_FLOAT  = 32
	DT_SLONG  = 64
	//
	DT_ULONGLONG = 128
)

// record represents a single telemetry record that contains
// an identifying code, a data type, and optional status attributes.
type record struct {
	code      uint16
	dataType  byte
	classType byte
	date      time.Time
	value     int64
	str       string
	fvalue    float64
	attribute []uint32
	attrVal   []byte
}

// SMA represents a single SMA inverter.
type SMA struct {
	name      string
	password  []byte
	conn      *net.UDPConn
	timeout   time.Duration
    genP      string
    volts     string
    genDaily  string
    genT     string
	appSusyid uint16
	appSerial uint32
	susyid    uint16
	serial    uint32
}

type request struct {
	packet_id uint16
	buf       *bytes.Buffer
}

var packet_id uint16 = 1

func init() {
	core.RegisterReader(smaReader)
}

func smaReader(conf *config.Config, wr chan<- core.Input) error {
	sect := conf.GetSection("sma")
	if sect == nil {
		return nil
	}
	log.Printf("Registered SMA inverter reader\n")
	// Inverter name is of the format [IP address|name]:port
	inverter, err := sect.GetArg("inverter")
	if err != nil {
		return err
	}
	password, err := sect.GetArg("inverter-password")
	if err != nil {
		return err
	}
	sma, err := NewSMA(inverter, password)
	if err != nil {
		return err
	}
	go sma.run(wr)
	return nil
}

func NewSMA(inverter string, password string) (*SMA, error) {
	raddr, err := net.ResolveUDPAddr("udp4", inverter)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialUDP("udp4", nil, raddr)
	if err != nil {
		return nil, err
	}
	// The password is encoded.
	pb := bytes.NewBufferString(password).Bytes()
	var enc []byte
	for i := 0; i < password_length; i++ {
		var c byte
		if i < len(pb) {
			c = pb[i]
		}
		enc = append(enc, c+password_enc)
	}
	s := &SMA{name: inverter, password: enc, conn: conn}
	s.appSusyid = 125
	s.appSerial = 900000000 + uint32(rand.Intn(100000000))
	s.genP = core.AddSubGauge(core.G_GEN_P, false)
	s.volts = core.AddSubGauge(core.G_VOLTS, true)
	s.genDaily = core.AddSubAccum(core.A_GEN_DAILY, true)
	s.genT = core.AddSubAccum(core.A_GEN_TOTAL, false)
	return s, nil
}

func (s *SMA) run(wr chan<- core.Input) {
	defer s.conn.Close()
	for {
		hour := time.Now().Hour()
		err := s.poll(wr, hour >= *core.StartHour && hour < *core.EndHour)
		if err != nil {
			log.Printf("Inverter poll error:%s - %v", s.name, err)
		}
		time.Sleep(time.Duration(*smapoll) * time.Second)
	}
}

func (s *SMA) poll(wr chan<- core.Input, daytime bool) error {
	if *core.Verbose {
		log.Printf("Polling inverter %s", s.name)
	}
	_, _, err := s.Logon()
	if err != nil {
		return err
	}
	defer s.Logoff()
	d, t, err := s.Energy()
	if err != nil {
		return err
	}
	if *core.Verbose {
		log.Printf("Tag %s Daily yield = %f, tag %s total yield = %f", s.genDaily, d, s.genT, t)
	}
	wr <- core.Input{Tag: s.genDaily, Value: d}
	wr <- core.Input{Tag: s.genT, Value: t}
	if daytime {
		v, err := s.Voltage()
		if err != nil {
			return err
		}
		if v != 0 {
			if *core.Verbose {
				log.Printf("Tag %s volts = %f", s.volts, v)
			}
			wr <- core.Input{Tag: s.volts, Value: v}
		}
		p, err := s.Power()
		if err != nil {
			return err
		}
		if p != 0 {
			pf := float64(p) / 1000
			if *core.Verbose {
				log.Printf("Tag %s power = %f", s.genP, pf)
			}
			wr <- core.Input{Tag: s.genP, Value: pf}
		}
	}
	return nil
}

// Initialise the connection to the inverter.
func (s *SMA) Logon() (uint16, uint32, error) {
	s.susyid = 0xFFFF
	s.serial = 0xFFFFFFFF
	req, err := s.cmdpacket(0x00000200, 0, 0)
	if err != nil {
		return 0, 0, err
	}
	b, err := s.response(req)
	if err != nil {
		return 0, 0, err
	}
	s.Logoff()
	// Now logon to the inverter.
	r := s.packet(14, 0xA0, 0x100)
	binary.Write(r.buf, binary.LittleEndian, uint32(0xFFFD040C))
	binary.Write(r.buf, binary.LittleEndian, uint32(0x7))               // group = USER
	binary.Write(r.buf, binary.LittleEndian, uint32(900))               // Timeout
	binary.Write(r.buf, binary.LittleEndian, uint32(time.Now().Unix())) // Time
	binary.Write(r.buf, binary.LittleEndian, uint32(0))                 // ?
	r.buf.Write(s.password)
	err = s.send(r)
	if err != nil {
		return 0, 0, err
	}
	b, err = s.response(r)
	if err != nil {
		return 0, 0, err
	}
    pkt := b.Bytes()
	retCode := binary.LittleEndian.Uint16(pkt[36:])
	if retCode == 0x0100 {
		return 0, 0, fmt.Errorf("Invalid password")
	} else if retCode != 0 {
		return 0, 0, fmt.Errorf("Logon faied, retCode = %04x", retCode)
	}
	s.susyid = binary.LittleEndian.Uint16(pkt[28:])
	s.serial = binary.LittleEndian.Uint32(pkt[30:])
	if *core.Verbose {
		log.Printf("Successful logon to inverter, susyid = %d, serial = %d",
			s.susyid, s.serial)
	}
	return s.susyid, s.serial, nil
}

func (s *SMA) Logoff() error {
	r := s.packet(8, 0xA0, 0x300)
	binary.Write(r.buf, binary.LittleEndian, 0xFFFD010E)
	binary.Write(r.buf, binary.LittleEndian, 0xFFFFFFFF)
	return s.send(r)
}

func (s *SMA) DeviceStatus() (string, error) {
	req, err := s.cmdpacket(0x51800200, 0x00214800, 0x002148FF)
	if err != nil {
		return "", err
	}
	b, err := s.response(req)
	if err != nil {
		return "", err
	}
	rec := unpackRecords(b)
	if *trace {
		dumpRecords("Device status", rec)
	}
	r, ok := rec[0x2148]
	if !ok {
		return "", fmt.Errorf("Unknown status record")
	}
	var status string
	for i, at := range r.attribute {
		switch at {
		case 35:
			status = status + fmt.Sprintf("[Alarm: %d]", r.attrVal[i])
		case 303:
			status = status + fmt.Sprintf("[Off: %d]", r.attrVal[i])
		case 307:
			status = status + fmt.Sprintf("[OK: %d]", r.attrVal[i])
		case 455:
			status = status + fmt.Sprintf("[Warning: %d]", r.attrVal[i])
		default:
			status = status + fmt.Sprintf("Unknown(%d): %d]", at, r.attrVal[i])
		}
	}
	return status, nil
}

func (s *SMA) Voltage() (float64, error) {
	recs, err := s.getRecords(0x51000200, 0x00464800, 0x004655FF)
	if err != nil {
		return 0, err
	}
	r, ok := recs[0x4648]
	if !ok {
		return 0, fmt.Errorf("Bad records")
	}
	return float64(r.value) / 100, nil
}

// Return daily yield and total yield in KwH
func (s *SMA) Energy() (float64, float64, error) {
	recs, err := s.getRecords(0x54000200, 0x00260100, 0x002622FF)
	if err != nil {
		return 0, 0, err
	}
	daily, ok := recs[0x2622]
	if !ok {
		return 0, 0, fmt.Errorf("Bad records")
	}
	total, ok := recs[0x2601]
	if !ok {
		return 0, 0, fmt.Errorf("Bad records")
	}
	return float64(daily.value) / 1000, float64(total.value) / 1000, nil
}

func (s *SMA) Power() (int64, error) {
	recs, err := s.getRecords(0x51000200, 0x00263F00, 0x00263FFF)
	if err != nil {
		return 0, err
	}
	var total int64
	for _, c := range []uint16{0x263F} {
		p, ok := recs[c]
		if ok {
			total += p.value
		}
	}
	return total, nil
}

func (s *SMA) processRecords(code, a1, a2 uint32) (int, error) {
	recs, err := s.getRecords(code, a1, a2)
	if err != nil {
		return 0, err
	}
	return len(recs), nil
}

func (s *SMA) getRecords(code, a1, a2 uint32) (map[uint16]*record, error) {
	req, err := s.cmdpacket(code, a1, a2)
	if err != nil {
		return nil, err
	}
	b, err := s.response(req)
	if err != nil {
		return nil, err
	}
	m := unpackRecords(b)
	if *trace {
		dumpRecords("GetRecords", m)
	}
	return m, nil
}

func (s *SMA) cmdpacket(cmd, first, last uint32) (*request, error) {
	r := s.packet(9, 0xA0, 0)
	binary.Write(r.buf, binary.LittleEndian, cmd)
	binary.Write(r.buf, binary.LittleEndian, first)
	binary.Write(r.buf, binary.LittleEndian, last)
	err := s.send(r)
	if err != nil {
		return nil, err
	}
	return r, nil
}

// Read the packet from the inverter and verify it.
func (s *SMA) response(req *request) (*bytes.Buffer, error) {
    tout := time.Now().Add(time.Duration(*smatimeout) * time.Second)
	for {
		b, err := s.read(tout.Sub(time.Now()))
		if err != nil {
			return nil, err
		}
		// Verify the packet and id.
		pkt := b.Bytes()
		if binary.LittleEndian.Uint32(pkt[14:]) != signature {
            log.Printf("Unknown signature, skipping packet\n")
			continue
		}
		rx_id := binary.LittleEndian.Uint16(pkt[40:])
		if rx_id != req.packet_id {
			log.Printf("RX id %04x, looking for %04x\n", rx_id, req.packet_id)
			continue
		}
		if *trace {
			log.Printf("Read buf, len %d\n", b.Len())
			dumpPacket(b)
		}
		return b, nil
	}
}

func (s *SMA) packet(longwords, c1 byte, c2 uint16) *request {
	var id uint16 = packet_id | 0x8000
	packet_id++
	b := new(bytes.Buffer)
	b.Write(packet_header)                            // 0
	binary.Write(b, binary.LittleEndian, signature)   // 14
	b.WriteByte(longwords)                            // 18 - longwords
	b.WriteByte(c1)                                   // 19 - control
	binary.Write(b, binary.LittleEndian, s.susyid)    // 20
	binary.Write(b, binary.LittleEndian, s.serial)    // 22
	binary.Write(b, binary.LittleEndian, c2)          // 26 control2
	binary.Write(b, binary.LittleEndian, s.appSusyid) // 28
	binary.Write(b, binary.LittleEndian, s.appSerial) // 30
	binary.Write(b, binary.LittleEndian, c2)          // 34 control2
	binary.Write(b, binary.LittleEndian, uint16(0))   // 36
	binary.Write(b, binary.LittleEndian, uint16(0))   // 38
	binary.Write(b, binary.LittleEndian, id)          // 40
	return &request{packet_id: id, buf: b}
}

func (s *SMA) send(r *request) error {
	// Write the trailer and set the length.
	binary.Write(r.buf, binary.LittleEndian, uint32(0)) // 54
	// Write the packet length into the buffer
	len := r.buf.Len() - (6 + len(packet_header))
	binary.BigEndian.PutUint16(r.buf.Bytes()[12:], uint16(len))

	if *trace {
		log.Printf("Sending pkt ID: %04x, length %d", r.packet_id, r.buf.Len())
		dumpPacket(r.buf)
	}
	n, err := s.conn.Write(r.buf.Bytes())
	if err != nil {
		return err
	}
	if n != r.buf.Len() {
		return fmt.Errorf("Write %d bytes of buffer size %d", n, r.buf.Len())
	}
	return nil
}

func (s *SMA) read(timeout time.Duration) (*bytes.Buffer, error) {
	s.conn.SetReadDeadline(time.Now().Add(timeout))
	b := make([]byte, maxPacketSize)
	n, err := s.conn.Read(b)
	if err != nil {
		return nil, err
	}
	return bytes.NewBuffer(b[:n]), nil
}

// Unpack records from the buffer.
func unpackRecords(b *bytes.Buffer) map[uint16]*record {
	m := make(map[uint16]*record)
	// Skip headers.
	b.Next(54)
	if *trace {
		log.Printf("Unpacking:")
		dumpPacket(b)
	}
	for b.Len() >= 8 {
		r := new(record)
		r.classType, _ = b.ReadByte()
		binary.Read(b, binary.LittleEndian, &r.code)
		r.dataType, _ = b.ReadByte()
		var t uint32
		binary.Read(b, binary.LittleEndian, &t)
		r.date = time.Unix(int64(t), 0)
		done := 8
		var size int
		switch r.code {
		// 16 byte records
		case 0x2601, 0x2622:
			size = 16
		// 28 byte records
		case 0x251E, 0x263F, 0x411E, 0x411F, 0x4120, 0x4640, 0x4641,
			0x4642, 0x4648, 0x4649, 0x464A, 0x4650, 0x4651, 0x4652,
			0x4653, 0x4654, 0x4655:
			size = 28

		// 40 byte records
		case 0x2148:
			size = 40

		// Unknown.
		default:
			log.Printf("code %04x length not known, aborting\n", r.code)
			return m
		}
		switch r.code {
		// These codes have a 64 bit value.
		case 0x2622, 0x2601, 0x462F, 0x462E:
			r.dataType = DT_ULONGLONG
		}
		switch r.dataType {
		case DT_ULONG:
			var v uint32
			binary.Read(b, binary.LittleEndian, &v)
			if v == nan32 || v == nanu32 {
				v = 0
			}
			r.value = int64(v)
			done += 4
		case DT_SLONG:
			var v uint32
			binary.Read(b, binary.LittleEndian, &v)
			if v == nan32 || v == nanu32 {
				v = 0
			}
			r.value = int64(int32(v))
			done += 4
		case DT_ULONGLONG:
			var v uint64
			binary.Read(b, binary.LittleEndian, &v)
			if v == nan64 || v == nanu64 {
				v = 0
			}
			r.value = int64(v)
			done += 8
		case DT_STATUS:
			for done < size {
				done += 4
				var a uint32
				binary.Read(b, binary.LittleEndian, &a)
				if a == 0xFFFFFE {
					break
				}
				r.attribute = append(r.attribute, a&0xFFFFFF)
				r.attrVal = append(r.attrVal, byte(a>>24))
			}
		default:
			log.Printf("Data type %02x not known, aborting\n", r.code)
			return m
		}
		b.Next(size - done)
		if *trace {
			log.Printf("Rec # %d, code: %04x, record size %d", len(m), r.code, size)
		}
		m[r.code] = r
	}
	return m
}

func dumpPacket(b *bytes.Buffer) {
	var s string = "\n"
	for i, bt := range b.Bytes() {
		if i%16 == 0 {
			s = s + fmt.Sprintf("0x%04x: ", i)
		}
		if i%2 == 0 {
			s = s + " "
		}
		s = s + fmt.Sprintf("%02x", bt)
		if i%16 == 15 {
			s = s + "\n"
		}
	}
	log.Printf(s)
}

func dumpRecords(req string, recs map[uint16]*record) {
	log.Printf("Req: %s, records %d\n", req, len(recs))
	for _, r := range recs {
		log.Printf("Rec date %s code 0x%04x, data type %02x, class %02x, value %d\n",
			r.date.Format(time.RFC822), r.code, r.dataType,
			r.classType, r.value)
		for j, a := range r.attribute {
			log.Printf("Attribute %d, value %d", a, r.attrVal[j])
		}
	}
}
