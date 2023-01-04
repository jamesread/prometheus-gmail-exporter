package main

import (
	log "github.com/sirupsen/logrus"

	"github.com/jamesread/prometheus-gmail-exporter/pkg/gmail"
)

func init() {
	initLogging();
}

func initLogging() {
	log.SetFormatter(&log.TextFormatter{
		DisableColors: false,
		DisableTimestamp: true,
	})
}

func main() {
	log.Info("prometheus-gmail-exporter")

	gmail.UpdateLoop()
}

