package lcd

import (
    "log"
    "net"

    "github.com/aamcrae/config"
    "github.com/aamcrae/MeterMan/core"
)

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
    raddr, err := net.ResolveUDPAddr("udp4", inverter)
    if err != nil {
        return err
    }
    conn, err := net.DialUDP("udp4", nil, raddr)
    if err != nil {
        return err
    }
    go runSMA(conn, wr)
    return nil
}

func runSMA(conn *net.UDPConn, wr chan<- core.Input) {
    defer conn.Close()
    var c chan int
    for {
        <- c
    }
}
