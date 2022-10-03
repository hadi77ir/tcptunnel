package main

import (
	"flag"
	"os"
	"os/signal"
	"time"
)
import "github.com/sirupsen/logrus"

var (
	listenAddr        string
	targetAddr        string
	proxyAddr         string
	dialTimeout       int
	keepAliveInterval int
	showHelp          bool
	debugLog          bool
)

// Create a new instance of the logger. You can have any number of instances.
var log = logrus.New()

func init() {
	flag.BoolVar(&showHelp, "help", false, "show usage")
	flag.BoolVar(&debugLog, "debug", false, "more verbose logging")
	flag.StringVar(&listenAddr, "listen", "", "listening address (<host>:<port>)")
	flag.StringVar(&targetAddr, "target", "", "remote target (<host>:<port>)")
	flag.StringVar(&proxyAddr, "proxy", "", "proxy address (<proto>://[user[:password]@]<host>:<port>/)")
	flag.IntVar(&dialTimeout, "timeout", 10, "dial timeout")
	flag.IntVar(&keepAliveInterval, "keepalive", 30, "keep-alive interval")
}

func main() {
	flag.Parse()

	log.SetLevel(logrus.InfoLevel)
	if debugLog {
		log.SetLevel(logrus.DebugLevel)
	}

	log.Debugf("logging level set to %s", log.GetLevel())

	if showHelp || targetAddr == "" || listenAddr == "" {
		flag.Usage()
		return
	}
	signals := make(chan os.Signal, 1)

	signal.Notify(signals, os.Interrupt, os.Kill)
	client := newClient(listenAddr, targetAddr, proxyAddr, time.Duration(dialTimeout), time.Duration(keepAliveInterval), signals)
	err := client.Run()
	if err != nil {
		log.Fatalf("exiting on error: %s", err)
	}
}
