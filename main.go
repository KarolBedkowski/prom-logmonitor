// Copyright (C) Karol Będkowski, 2017
//
// Distributed under terms of the GPLv3 license.
package main

import (
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"

	"github.com/Merovius/systemd"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
)

var (
	showVersion = flag.Bool("version", false, "Print version information.")
	configFile  = flag.String("config.file", "logmonitor.yml",
		"Path to configuration file.")
	listenAddress = flag.String("web.listen-address", ":9704",
		"Address to listen on for web interface and telemetry.")
)

func init() {
	prometheus.MustRegister(version.NewCollector("logmonitor"))
}

func main() {
	flag.Parse()

	if *showVersion {
		fmt.Fprintln(os.Stdout, version.Print("logmonitor"))
		os.Exit(0)
	}

	systemd.NotifyStatus("starting")
	systemd.AutoWatchdog()

	log.Infoln("Starting logmonitor", version.Info())
	log.Infoln("Build context", version.BuildContext())

	c, err := LoadConfiguration(*configFile)
	if err != nil {
		log.Fatalf("Error parsing config file: %s", err)
		return
	}

	http.Handle("/metrics", promhttp.Handler())

	monitors := createMonitors(c)

	// handle hup for reloading configuration
	hup := make(chan os.Signal)
	signal.Notify(hup, syscall.SIGHUP)
	go func() {
		for {
			select {
			case <-hup:
				systemd.NotifyStatus("reloading")
				if newConf, err := LoadConfiguration(*configFile); err == nil {
					log.Debugf("new configuration: %+v", newConf)
					c = newConf

					for _, m := range monitors {
						m.Stop()
					}
					monitors = createMonitors(c)

					log.Info("configuration reloaded")
				} else {
					log.Errorf("reloading configuration err: %s", err)
					log.Errorf("using old configuration")
				}
			}
		}
	}()

	// cleanup
	cleanChannel := make(chan os.Signal, 1)
	signal.Notify(cleanChannel, os.Interrupt, syscall.SIGTERM, syscall.SIGKILL)
	go func() {
		<-cleanChannel
		log.Info("Closing...")
		systemd.Notify("STOPPING=1\r\nSTATUS=stopping")
		for _, m := range monitors {
			m.Stop()
		}
		systemd.NotifyStatus("stopped")
		os.Exit(0)
	}()

	go func() {
		log.Infof("Listening on %s", *listenAddress)
		log.Fatal(http.ListenAndServe(*listenAddress, nil))
	}()

	systemd.NotifyReady()
	systemd.NotifyStatus("running")

	done := make(chan bool)
	<-done
}

func createMonitors(c *Configuration) (monitors []*Monitor) {
	for _, l := range c.Files {
		if !l.Enabled {
			continue
		}
		m, err := NewMonitor(l)
		if err != nil {
			log.Errorf("Creating monitor %s error: %s", l.File, err)
			continue
		}
		monitors = append(monitors, m)
		if err := m.Start(); err != nil {
			log.Errorf("Start monitor %s error: %s", l.File, err)
		}
	}
	return
}
