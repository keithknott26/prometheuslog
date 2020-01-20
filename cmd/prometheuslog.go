package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/as/hue"
	prometheusmetrics "github.com/deathowl/go-metrics-prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rcrowley/go-metrics"
	kingpin "gopkg.in/alecthomas/kingpin.v2"

	prometheuslog "github.com/keithknott26/prometheuslog/pkg/app"
)

var (
	app                  = kingpin.New("prometheus-log", "Expose Metrics endpoint to Prometheus/Grafana")
	debug                = app.Flag("debug", "Enable Debug Mode").Bool()
	port                 = app.Flag("port", "Port to listen for metrics requests. Default: 9091").Short('p').Default("9091").Int()
	environment          = app.Flag("environment", "Environment (staging, uat, or prod). Default: prod").Short('e').Default("prod").String()
	metricsFlushInterval = app.Flag("flush-interval", "How often to flush available metrics: (1s,5s,15s,1h,etc) (default: 2s) ...)").Short('f').Default("2s").Duration()
	maxIngestionRate     = app.Flag("max-ingestion-rate", "Ingestion Rate Limiter:(1000,5000,10000,etc) in operations per/sec (default: 10000) ...)").Short('r').Default("10000").Int()
	configFile           = app.Flag("config-file", "Full path to the prometheuslog.conf config file.\n").Short('c').ExistingFile()
)

type property struct {
	name string
	log  string
}

func readCSVFile(fileName string) ([][]string, error) {
	file, err := os.Open(fileName)
	if err != nil {
		log.Fatal(fmt.Sprintf("Failed to open the conf file: %s\n", fileName))
	}
	r := csv.NewReader(file)
	lines, err := r.ReadAll()
	if err != nil {
		log.Fatal("Failed to parse the provided conf file.")
	}
	return lines, err
}

func parseCSVLines(lines [][]string) []property {
	ret := make([]property, len(lines))
	for i, line := range lines {
		ret[i] = property{
			name: line[0],
			log:  line[1],
		}
	}
	return ret
}
func serveEndpoint() {
	http.Handle("/metrics", promhttp.Handler())
	portNumber := strconv.Itoa(*port)
	portStr := fmt.Sprintf(":%s", portNumber)
	green := hue.New(hue.Green, hue.Default)
	hw := hue.NewWriter(os.Stdout, green)
	hw.SetHue(green)
	hw.WriteString(fmt.Sprintf("Listening for /metrics requests on port %s\n", portStr))
	log.Fatal(http.ListenAndServe(portStr, nil))
}
func enableMetricsLogging(applicationName string, registry metrics.Registry, intervalSec time.Duration) {
	/*         Metric Logging              */
	/*  uncomment to log metrics to a file */
	filename := fmt.Sprintf("%s.metrics.log", applicationName)
	logfile, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	//defer logfile.Close()

	log.SetOutput(logfile)
	title := fmt.Sprintf("%s:\t", applicationName)
	go metrics.Log(registry, intervalSec, log.New(logfile, title, log.Lmicroseconds))
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func directoryExists(path string) bool {
	// check if the source dir exist
	src, err := os.Stat(path)
	if err != nil {
		return false
	}

	// check if the source is indeed a directory or not
	if src.IsDir() {
		return true
	}
	return false
}

// Publish the port as soon as the program starts
func init() {

}

func main() {

	// Parse args and assign values
	kingpin.Version("0.0.1")
	kingpin.MustParse(app.Parse(os.Args[1:]))

	//assign colors used in text output
	red := hue.New(hue.Red, hue.Default)
	green := hue.New(hue.Green, hue.Default)
	blue := hue.New(hue.Blue, hue.Default)
	yellow := hue.New(hue.Brown, hue.Default)
	magenta := hue.New(hue.Magenta, hue.Default)
	// Print a green string with a hue.Writer
	hw := hue.NewWriter(os.Stdout, green)

	// create new application
	App := prometheuslog.NewApp()

	//whether or not to enable debug messages
	var debugEnabled bool
	if *debug == true {
		debugEnabled = true
	} else {
		debugEnabled = false
	}
	//set the max ingestion rate
	maxRate := *maxIngestionRate

	//check if config file is specified
	if *configFile == "" {
		fmt.Println("You did not specify a config file, exiting...")
		os.Exit(1)
	} else {
		// start CSV file processing
		hw.WriteString("Parsing config file...\n")
		lines, err := readCSVFile(*configFile)
		if err != nil {
			hw.SetHue(red)
			hw.WriteString("Error locating prometheuslog.conf, did you provide the full path?")
		}
		//parse config file into individual instances
		instances := parseCSVLines(lines)
		hw.WriteString("Creating objects and applying metrics configuration..\n")
		for id, app := range instances {

			if _, err := os.Stat(app.log); err == nil {
				hw.SetHue(green)
				hw.WriteString("Adding:  ")
				hw.SetHue(red)
				hw.WriteString("ID: ")
				hw.SetHue(yellow)
				hw.WriteString(fmt.Sprintf("%d", id))
				hw.SetHue(blue)
				hw.WriteString("\tInstance Name: ")
				hw.SetHue(magenta)
				hw.WriteString(fmt.Sprintf("%s", app.name))
				hw.SetHue(blue)
				hw.WriteString("\tLog: ")
				hw.SetHue(yellow)
				hw.WriteString(fmt.Sprintf("%s\n", app.log))
				/*
					Variables from Config file:
					applications.name
					applications.log
				*/

				Application := App.AddApplication(id, app.name, app.log, maxRate, debugEnabled)
				if debugEnabled == true {
					enableMetricsLogging(app.name, Application.MetricsRegistry, 60*time.Second)
				}

				//sleep for a second
				time.Sleep(1 * time.Second)

				pClient := prometheusmetrics.NewPrometheusProvider(Application.MetricsRegistry, app.name, *environment, prometheus.DefaultRegisterer, *metricsFlushInterval)
				go pClient.UpdatePrometheusMetrics()

			} else if os.IsNotExist(err) {
				hw.SetHue(red)
				hw.WriteString(fmt.Sprintf("Skipping (file doesn't exist): %s", app.name))
			} else {
				hw.SetHue(red)
				hw.WriteString(fmt.Sprintf("Skipping: %s", app.name))
			}
		}
	}
	hw.SetHue(green)
	hw.WriteString(fmt.Sprintf("\n\nService Started...\n"))
	go serveEndpoint()

	/* Gracefully exit the program */
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt)
	go func() {
		select {
		case sig := <-c:
			hw.SetHue(red)
			hw.WriteString(fmt.Sprintf("Got %s signal. Aborting service sanely...\n", sig))
			os.Exit(1)
		}
	}()

	metricsInterval := 30 * time.Second
	tick := time.Tick(metricsInterval)
	for {
		select {
		case <-tick:
			fmt.Printf(".")
		}
	}
}
