package sma

import (
    "bytes"
    "encoding/binary"
    "flag"
    "log"
    "net"
    "time"

    "github.com/aamcrae/config"
    "github.com/aamcrae/MeterMan/core"
)

var smatimeout = flag.Int("inverter_timeout", 10, "Inverter timeout in seconds")

const maxPacketSize = 8 * 1024
const packet_header []bytes = []bytes{ 'S', 'M', 'A', 0, 0, 0x04, 0x02, 0xA0,
                                        0, 0, 0, 0x01, 0, 0}

const (
    GET_ENERGY = iota
    SW_VERSION
    GET_STATUS
)

type SMA struct {
    name string
    conn *net.UDPConn
    timeout time.Duration
    uint16 susyid
    uint32 serial
}

type request struct {
    packet_id uint16
    buf *bytes.Buffer
}

var packet_id uint16 = 1

func init() {
    core.RegisterReader(smaReader)
}

func smaReader(conf *config.Config, wr chan<- core.Input) error {
    log.Printf("Registered SMA inverter reader\n")
    // Inverter name is of the format [IP address|name]:port
    inverter, err := conf.GetArg("inverter")
    if err != nil {
        return err
    }
    sma, err := NewSMA(inverter)
    go sma.poll(wr)
    return nil
}

func NewSMA(inverter string) (* SMA, error) {
    raddr, err := net.ResolveUDPAddr("udp4", inverter)
    if err != nil {
        return nil, err
    }
    conn, err := net.DialUDP("udp4", nil, raddr)
    if err != nil {
        return nil, err
    }
    return &SMA{name:inverter, conn:conn}, nil
}

func (s *SMA) poll(wr chan<- core.Input) {
    err := s.query()
    if err != nil {
        log.Fatalf("Cannot contact inverter: %s: %v", s.name, err)
    }
    defer s.conn.Close()
    var c chan int
    <-c
}

func (s *SMA) Query() (*bytes.Buffer, error) {
    s.susid = 0xFFFF
    s.serial = 0xFFFFFFFF
    req, err := s.cmdpacket(0x00000200, 0, 0)
    if err != nil {
        return err
    }
    return s.response(req)
}

func (s *SMA) device_status() error {
    req, err := s.cmdpacket(0x51800200, 0x00214800, 0x002148FF)
    if err != nil {
        return err
    }
    b, err := s.response(req)
}

func (s *SMA) cmdpacket(cmd, first, last uint32) *request {
    var id uint16 = packet_id++
    b := new(bytes.Buffer)
    b.Write(packet_header)
    binary.Write(b, binary.LittleEndian, uint32(0x65601000))
    b.WriteByte(9)      // longwords
    b.WriteByte(0xA0)   // control
    binary.Write(b, binary.LittleEndian, s.susyid)
    binary.Write(b, binary.LittleEndian, s.serial)
    binary.Write(b, binary.LittleEndian, uint16(0))  // control2
    binary.Write(b, binary.LittleEndian, uint16(0))
    binary.Write(b, binary.LittleEndian, uint16(0))
    binary.Write(b, binary.LittleEndian, id | 0x8000

    binary.Write(b, binary.LittleEndian, cmd)
    binary.Write(b, binary.LittleEndian, first)
    binary.Write(b, binary.LittleEndian, last)
    binary.Write(b, binary.LittleEndian, uint32(0))
    // Write the packet length into the buffer
    len := b.Len() - (6 + len(packet_header))
    binary.BigEndian.PutUint16(b.Bytes()[12:], uint16(len))

    n, err := s.conn.Write(b.Bytes())
    if err != nil {
        return nil, err
    }
    if n != b.Len() {
        return nil, fmt.Errorf("Write %d bytes of buffer size %d", n, b.Len())
    }
    return &request{packet_id:packet_id++, buf:b}, nil
}

// Read the packet from the inverter and verify it.
func (s *SMA) response(req request) (*bytes.Buffer, error) {
    for {
        b, err := s.read(time.Second * *smatimeout)
        if err != nil {
            return nil, err
        }
        // Verify the packet id.
        return b, nil
    }
}

// Flush any old packets out of the socket.
func (s *SMA) flush() {
    for {
        _, err := s.read(time.Millisecond * 50)
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
        return nil, err
    }
    return bytes.NewBuffer(b[:n]), nil
}
