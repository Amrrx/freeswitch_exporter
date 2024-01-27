package main

import (
	"fmt"

	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"net/http"
	"os"
	"strings"

	"github.com/prometheus/exporter-toolkit/web"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	cnfc_uuid = os.Getenv("CNFC_UUID")
	cnf_uuid  = os.Getenv("CNF_UUID")
	ns_uuid   = os.Getenv("NS_UUID")
)

func main() {
	var (
		listenAddress = kingpin.Flag(
			"web.listen-address",
			"Address to listen on for web interface and telemetry.").Short('l').Default(":9282").String()
		metricsPath = kingpin.Flag(
			"web.telemetry-path",
			"Path under which to expose metrics.").Default("/metrics").String()
		scrapeURI = kingpin.Flag(
			"core_sbc.scrape-uri",
			`URI on which to scrape core_sbc. E.g. "tcp://localhost:8021"`).Short('u').Default("tcp://localhost:8021").String()
		timeout = kingpin.Flag(
			"core_sbc.timeout",
			"Timeout for trying to get stats from core_sbc.").Short('t').Default("5s").Duration()
		password = kingpin.Flag(
			"core_sbc.password",
			"Password for core_sbc event socket.").Short('P').Default("ClueCon").String()
		configFile = kingpin.Flag(
			"web.config",
			"[EXPERIMENTAL] Path to config yaml file that can enable TLS or authentication.",
		).Default("").String()
		rtpEnable = kingpin.Flag("rtp.enable", "enable rtp info, default: false").Default("false").Bool()
	)
	logLevel := &AllowedLevel{}
	logLevel.Set("debug")

	jsonFormat := &AllowedFormat{}
	jsonFormat.Set("json")

	promlogConfig := &Config{Format: jsonFormat,
		Level: logLevel}

	kingpin.Version("core_sbc_exporter")
	logger := New(promlogConfig)

	kingpin.Parse()

	c, err := NewCollector(*scrapeURI, *timeout, *password, *rtpEnable)

	if err != nil {
		panic(err)
	}

	prometheus.MustRegister(c)

	http.Handle(*metricsPath, promhttp.Handler())

	// This implements Prometheus' multi-target exporter support
	// Example project: Official Blackbox Exporter
	// https://github.com/prometheus/blackbox_exporter#prometheus-configuration
	http.HandleFunc("/probe", func(w http.ResponseWriter, r *http.Request) {
		target := r.URL.Query().Get("target")
		if target == "" {
			http.Error(w, "'target' query param not provided, but required.", http.StatusBadRequest)

		}

		// Not checking for the port to allow the port to be configured in
		// the Prometheus scrape target config.
		if !strings.HasPrefix(target, "tcp://") {
			target = fmt.Sprintf("tcp://%s", target)
		}

		c, colErr := NewCollector(target, *timeout, *password, *rtpEnable)
		if colErr != nil {
			http.Error(w, fmt.Sprintf("failed to create collector for %s: %s", target, colErr), http.StatusInternalServerError)
		}

		registry := prometheus.NewRegistry()
		registry.MustRegister(c)

		promhttp.HandlerFor(registry, promhttp.HandlerOpts{}).ServeHTTP(w, r)

	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
			<head><title>CourzadSBC Exporter</title></head>
			<body>
			<h1>CourzadSBC Exporter</h1>
			<p><a href="` + *metricsPath + `">Metrics</a></p>
			</body>
			</html>`))
	})

	// In the New and NewDynamic functions

	// In the SetLevel method
	// _ = l.base.Log("message", "Log level changed", "prev", l.currentLevel, "current", lvl)

	level.Info(logger).Log("message", "Listening on", "address", *listenAddress)
	server := &http.Server{Addr: *listenAddress}
	if err := web.ListenAndServe(server, *configFile, logger); err != nil {
		level.Info(logger).Log("err", err)
		os.Exit(1)
	}
}
