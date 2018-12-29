package core

import (
    "flag"

    "github.com/aamcrae/config"
)


var Verbose = flag.Bool("verbose", false, "Verbose tracing")

type Result struct {
    Tag string
    Value float64
}

var writersInit []func (*config.Config) (chan<- Result, error)
var readersInit []func (*config.Config, []chan<-Result) error

func RegisterWriter(f func (*config.Config) (chan<- Result, error)) {
    writersInit = append(writersInit, f)
}

func RegisterReader(f func (*config.Config, []chan<- Result) error) {
    readersInit = append(readersInit, f)
}

func SetUp(conf *config.Config) error {
    var wr []chan<-Result
    for _, wi := range writersInit {
        if c, err := wi(conf); err != nil {
            return err
        } else {
            wr = append(wr, c)
        }
    }
    for _, ri := range readersInit {
        if err := ri(conf, wr); err != nil {
            return err
        }
    }
    return nil
}
