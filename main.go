package main

import (
	"flag"
	"log"
	"github.com/betabrandao/telnet-chat/config"
	"github.com/betabrandao/telnet-chat/server"
)

func init() {
	configFile := flag.String("config", "etc/config.json", "configuration file to parse")
	flag.Parse()

	log.Println("Reading configuration file:", *configFile)
	if err := config.ReadConfiguration(*configFile); err != nil {
		log.Fatalf("Failed to parse configuration file %q: %s", *configFile, err.Error())
	}
}

func main() {
	log.Println("Starting telnet chat server")

	s, err := server.NewServer()
	if err != nil {
		log.Fatalf("Failed to initialize TCP listener: %s", err.Error())
	}

	s.Serve()
}
