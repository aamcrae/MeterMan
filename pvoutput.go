package main

import (
    "flag"
    "log"
)

var pvoutput = flag.String("pvoutput", ".meterman", "Config file")
var input chan Result = make(chan Result, 100)

func init() {
    Writers = append(Writers, input)
    go pvread(input)
}

func pvread(in chan Result) {
    for {
        r := <-in
        process(r.tag, r.value)
    }
}

func process(tag string, value float64) {
    if *verbose {
        log.Printf("pvoutput: Tag: %s, value %f\n", tag, value)
    }
    switch tag {
    case "IN":
    case "OUT":
    case "TP":
    }
}
