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

// package sma implements reading telemetry data from a SMA SunnyBoy solar inverter.

package sma

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"sync/atomic"
	"time"
)

var smatimeout = flag.Int("inverter-timeout", 10, "Inverter timeout in seconds")
var trace = flag.Bool("inverter-trace", false, "Enable trace of record processing")
var pktTrace = flag.Bool("packet-trace", false, "Enable packet dumps")

const maxPacketSize = 8 * 1024

var packet_header = []byte{'S', 'M', 'A', 0, 0, 0x04, 0x02, 0xA0, 0, 0, 0, 0x01, 0, 0}

const signature = uint32(0x65601000)

const password_length = 12
const password_enc = 0x88

// Data types of record value.
const (
	DT_ULONG     = 0   // Unsigned 32 bits
	DT_STATUS    = 8   // upper 8 bits are attribute value, lower 24 bits attribute ID.
	DT_STRING    = 16  // 32 bytes null terminated string
	DT_FLOAT     = 32  // 32 bit float (unused)
	DT_SLONG     = 64  // Signed 32 bits
	DT_ULONGLONG = 128 // Unsigned 64 bits
)

// Special meanings of record values.
const nan64 = 0x8000000000000000
const nan32 = 0x80000000
const nanu64 = 0xFFFFFFFFFFFFFFFF
const nanu32 = 0xFFFFFFFF

// Commands to inverter for retrieving records.
const (
	CMD_INV_LOGON = 0x00000200 // Login to inverter.
	CMD_INV_40    = 0x58000200 // Inverter status, 40 byte records
	CMD_SPOT_28   = 0x52000200 // Spot values, 28 byte records
	CMD_AC_16     = 0x54000200 // AC spot values, 16 byte records
	CMD_AC_28     = 0x51000200 // AC spot values, 28 byte records
	CMD_AC_40     = 0x51800200 // AC spot values, 40 byte records
	CMD_DC_28     = 0x53800200 // DC spot values, 28 byte records
)

// Map of commands to record size.
var cmdRecSize = map[uint32]int{
	CMD_INV_40:  40,
	CMD_SPOT_28: 28,
	CMD_AC_16:   16,
	CMD_AC_28:   28,
	CMD_AC_40:   40,
	CMD_DC_28:   28,
}

// record represents a single telemetry record that contains
// an identifying code, a data type, and optional status attributes.
type record struct {
	code      uint16
	dataType  byte
	classType byte
	date      time.Time
	value     int64
	str       string // If dataType is DT_STRING
	attribute []uint32
	attrVal   []byte
}

// SMA represents a single SMA SunnyBoy inverter.
type SMA struct {
	name      string       // device name or IP address
	password  []byte       // device password
	conn      *net.UDPConn // UDP connection
	timeout   time.Duration
	appSusyid uint16 // Application system ID
	susyid    uint16 // System ID from device
	serial    uint32 // Serial number of device
}

type request struct {
	packet_id uint16
	buf       *bytes.Buffer
}

var master_packet_id uint32
var appSerial uint32

func init() {
	rand.Seed(time.Now().UnixNano())
	appSerial = 900000000 + uint32(rand.Intn(100000000))
}

// NewSMA creates and initialises an object for accessing
// a SunnyBoy inverter.
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
	return s, nil
}

// Initialise the connection to the inverter.
func (s *SMA) Logon() (uint16, uint32, error) {
	s.susyid = 0xFFFF
	s.serial = 0xFFFFFFFF
	req, err := s.cmdPacket(CMD_INV_LOGON, 0, 0)
	if err != nil {
		return 0, 0, fmt.Errorf("%s: logon: %v", s.name, err)
	}
	b, err := s.response(req)
	if err != nil {
		return 0, 0, fmt.Errorf("logon: %v", err)
	}
	s.Logoff()
	s.flush()
	// logon to the inverter.
	r := s.packet(14, 0xA0, 0x100)
	binary.Write(r.buf, binary.LittleEndian, uint32(0xFFFD040C))
	binary.Write(r.buf, binary.LittleEndian, uint32(0x7))               // group = USER
	binary.Write(r.buf, binary.LittleEndian, uint32(900))               // Timeout
	binary.Write(r.buf, binary.LittleEndian, uint32(time.Now().Unix())) // Time
	binary.Write(r.buf, binary.LittleEndian, uint32(0))                 // ?
	r.buf.Write(s.password)
	err = s.send(r)
	if err != nil {
		return 0, 0, fmt.Errorf("logon: %v", err)
	}
	b, err = s.response(r)
	if err != nil {
		return 0, 0, fmt.Errorf("logon: %v", err)
	}
	pkt := b[0].Bytes()
	retCode := binary.LittleEndian.Uint16(pkt[36:])
	if retCode == 0x0100 {
		return 0, 0, fmt.Errorf("Invalid password")
	} else if retCode != 0 {
		return 0, 0, fmt.Errorf("Logon failed, retCode = %04x", retCode)
	}
	s.susyid = binary.LittleEndian.Uint16(pkt[28:])
	s.serial = binary.LittleEndian.Uint32(pkt[30:])
	if *trace {
		log.Printf("Successful logon to inverter %s, susyid = %d, serial = %d",
			s.name, s.susyid, s.serial)
	}
	return s.susyid, s.serial, nil
}

func (s *SMA) Logoff() error {
	r := s.packet(8, 0xA0, 0x300)
	binary.Write(r.buf, binary.LittleEndian, 0xFFFD010E)
	binary.Write(r.buf, binary.LittleEndian, 0xFFFFFFFF)
	return s.send(r)
}

func (s *SMA) Name() string {
	return s.name
}

func (s *SMA) Close() {
	s.conn.Close()
}

// Helper functions to retrieve specific values from the inverter.

// Retrieve the current status of the inverter.
func (s *SMA) DeviceStatus() (string, error) {
	recs, err := s.getRecords(CMD_AC_40, 0x00214800, 0x002148FF)
	if err != nil {
		return "", fmt.Errorf("device status: %v", err)
	}
	if *trace {
		dumpRecords("Device status", recs)
	}
	rl, ok := recs[0x2148]
	if !ok {
		return "", fmt.Errorf("device status: missing record")
	}
	r := rl[0]
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
	return s.getValue(CMD_AC_28, 0x4648, 100.0)
}

// Return daily yield in KwH
func (s *SMA) DailyEnergy() (float64, error) {
	return s.getValue(CMD_AC_16, 0x2622, 1000.0)
}

// Return total yield in KwH
func (s *SMA) TotalEnergy() (float64, error) {
	return s.getValue(CMD_AC_16, 0x2601, 1000.0)
}

// Return power in watts.
func (s *SMA) Power() (float64, error) {
	return s.getValue(CMD_AC_28, 0x263F, 1.0)
}

// Debug code to retrieve all records.
func (s *SMA) GetAll() error {
	codes := []uint32{CMD_AC_28, CMD_AC_40, CMD_INV_40, CMD_DC_28, CMD_AC_16, CMD_INV_40}
	for _, req := range codes {
		_, err := s.getRecords(req, 0x00000000, 0x00FFFFFF)
		if err != nil {
			return err
		}
	}
	return nil
}

// Get a scaled float value.
func (s *SMA) getValue(cmd uint32, id uint16, scale float64) (float64, error) {
	recId := uint32(id) << 8
	recs, err := s.getRecords(cmd, recId, recId|0xFF)
	if err != nil {
		return 0, err
	}
	v, ok := recs[id]
	if ok {
		return float64(v[0].value) / scale, nil
	} else {
		if *trace {
			log.Printf("%s: getValue: missing record (0x%04x)", s.name, id)
		}
		return 0, fmt.Errorf("getValue: missing record")
	}
}

// getRecords retrieves the requested records from the inverter,
// returning the records in a map keyed by the record ID.
func (s *SMA) getRecords(code, a1, a2 uint32) (map[uint16][]*record, error) {
	req, err := s.cmdPacket(code, a1, a2)
	if err != nil {
		return nil, fmt.Errorf("getRecords: %v", err)
	}
	b, err := s.response(req)
	if err != nil {
		return nil, fmt.Errorf("getrecords: %v", err)
	}
	m, err := unpackRecords(code, b)
	if err != nil {
		return nil, fmt.Errorf("unpackRecords: %v", err)
	}
	if *trace {
		dumpRecords("GetRecords", m)
	}
	return m, nil
}

// cmdPacket creates and sends a request to the inverter.
func (s *SMA) cmdPacket(cmd, first, last uint32) (*request, error) {
	r := s.packet(9, 0xA0, 0)
	binary.Write(r.buf, binary.LittleEndian, cmd)
	binary.Write(r.buf, binary.LittleEndian, first)
	binary.Write(r.buf, binary.LittleEndian, last)
	if *trace {
		log.Printf("%s: Sending cmd 0x%08x, first 0x%08x, last 0x%08x", s.name, cmd, first, last)
	}
	err := s.send(r)
	if err != nil {
		return nil, fmt.Errorf("cmdPacket: %v", err)
	}
	return r, nil
}

// response receives the response packet(s) from the inverter and verifies them.
func (s *SMA) response(req *request) ([]*bytes.Buffer, error) {
	tout := time.Now().Add(time.Duration(*smatimeout) * time.Second)
	var bList []*bytes.Buffer
	pkt_id := req.packet_id
	for {
		b, err := s.read(tout.Sub(time.Now()))
		if err != nil {
			return nil, fmt.Errorf("response: %v", err)
		}
		if *pktTrace {
			log.Printf("%s: Read buf, len %d", s.name, b.Len())
			dumpPacket(b)
		}
		// Check for minimum sized packet
		if b.Len() < 42 {
			log.Printf("%s: packet too short (%d), skipping packet", s.name, b.Len())
			continue
		}
		// Verify the packet and id.
		pkt := b.Bytes()
		if binary.LittleEndian.Uint32(pkt[14:]) != signature {
			log.Printf("%s: Unknown signature, skipping packet", s.name)
			continue
		}
		more := binary.LittleEndian.Uint16(pkt[38:])
		rx_id := binary.LittleEndian.Uint16(pkt[40:])
		if rx_id != pkt_id {
			log.Printf("%s: RX id %04x, looking for %04x", s.name, rx_id, pkt_id)
			continue
		}
		pkt_id &^= 0x8000
		bList = append(bList, b)
		if more == 0 {
			return bList, nil
		}
	}
}

// packet creates a packet header.
func (s *SMA) packet(longwords, c1 byte, c2 uint16) *request {
	new_id := atomic.AddUint32(&master_packet_id, 1)
	var id uint16 = uint16(new_id) | 0x8000 // Top bit set for first packet.
	b := new(bytes.Buffer)
	b.Write(packet_header)                            // 0
	binary.Write(b, binary.LittleEndian, signature)   // 14
	b.WriteByte(longwords)                            // 18 - longwords
	b.WriteByte(c1)                                   // 19 - control
	binary.Write(b, binary.LittleEndian, s.susyid)    // 20
	binary.Write(b, binary.LittleEndian, s.serial)    // 22
	binary.Write(b, binary.LittleEndian, c2)          // 26 control2
	binary.Write(b, binary.LittleEndian, s.appSusyid) // 28
	binary.Write(b, binary.LittleEndian, appSerial)   // 30
	binary.Write(b, binary.LittleEndian, c2)          // 34 control2
	binary.Write(b, binary.LittleEndian, uint16(0))   // 36
	binary.Write(b, binary.LittleEndian, uint16(0))   // 38 packet count following
	binary.Write(b, binary.LittleEndian, id)          // 40
	return &request{packet_id: id, buf: b}
}

func (s *SMA) send(r *request) error {
	// Write the trailer and set the length.
	binary.Write(r.buf, binary.LittleEndian, uint32(0)) // 54
	// Write the packet length into the buffer
	len := r.buf.Len() - (6 + len(packet_header))
	binary.BigEndian.PutUint16(r.buf.Bytes()[12:], uint16(len))

	if *pktTrace {
		log.Printf("%s: Sending pkt ID: %04x, length %d", s.name, r.packet_id, r.buf.Len())
		dumpPacket(r.buf)
	}
	n, err := s.conn.Write(r.buf.Bytes())
	if err != nil {
		return fmt.Errorf("send: %v", err)
	}
	if n != r.buf.Len() {
		return fmt.Errorf("wrote %d bytes of buffer size %d", n, r.buf.Len())
	}
	return nil
}

func (s *SMA) flush() {
	for {
		_, err := s.read(0)
		if err != nil {
			return
		}
	}
}

func (s *SMA) read(timeout time.Duration) (*bytes.Buffer, error) {
	s.conn.SetReadDeadline(time.Now().Add(timeout))
	b := make([]byte, maxPacketSize)
	n, err := s.conn.Read(b)
	if err != nil {
		return nil, fmt.Errorf("read: %v", err)
	}
	return bytes.NewBuffer(b[:n]), nil
}

// Unpack records from the buffer.
func unpackRecords(cmd uint32, bList []*bytes.Buffer) (map[uint16][]*record, error) {
	m := make(map[uint16][]*record)
	for pn, b := range bList {
		// Skip headers.
		b.Next(54)
		if *pktTrace {
			log.Printf("Unpacking pkt %d:", pn)
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
			size, ok := cmdRecSize[cmd]
			if !ok {
				return m, fmt.Errorf("Unknown size for cmd 0x%04x", cmd)
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
			case DT_STRING:
				r.str = string(bytes.Trim(b.Next(32), "\x00"))
				done += 32
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
				return m, fmt.Errorf("cmd 0x%08x code 0x%04x, unknown data type: 0x%02x", cmd, r.code, r.dataType)
			}
			b.Next(size - done)
			if *trace {
				log.Printf("Rec # %d, code: %04x, record size %d", len(m), r.code, size)
			}
			m[r.code] = append(m[r.code], r)
		}
	}
	return m, nil
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

func dumpRecords(req string, recs map[uint16][]*record) {
	log.Printf("Req: %s, records %d\n", req, len(recs))
	for _, rs := range recs {
		for _, r := range rs {
			var v string
			if r.dataType == DT_STRING {
				v = fmt.Sprintf("<%s>", r.str)
			} else {
				v = fmt.Sprintf("%d", r.value)
			}
			log.Printf("Rec date %s code 0x%04x, data type %02x, class %02x, value %s\n",
				r.date.Format(time.RFC822), r.code, r.dataType, r.classType, v)
			for j, a := range r.attribute {
				log.Printf("Attribute %d, value %d", a, r.attrVal[j])
			}
		}
	}
}
