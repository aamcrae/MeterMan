package main

import (
	"flag"
	"log"

	"github.com/aamcrae/MeterMan/temp"
)

var url = flag.String("weather", "", "Weather URL")

func init() {
	flag.Parse()
}

func main() {

	t, err := temp.GetTemp(*url)
	if err != nil {
		log.Fatalf("%s: %v", *url, err)
	}
    log.Printf("Temperature is currently %f degrees\n", t)
}
