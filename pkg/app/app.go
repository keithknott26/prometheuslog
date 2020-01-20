package prometheuslog

import (
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	prometheusmetrics "github.com/deathowl/go-metrics-prometheus"
	"github.com/papertrail/go-tail/follower"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rcrowley/go-metrics"
	"go.uber.org/ratelimit"
)

var (
	mutex = &sync.Mutex{}
)

type App struct {
	sync.Mutex
	MetricsShipFrequency int
	TotalLinesRead       int
	LogTimeDifference    string
	Applications         []Application
}

type Application struct {
	App
	sync.Mutex
	ID                 int
	TotalLinesRead     int
	ReadRate           int
	LogTimeDifference  string
	ApplicationName    string
	LogPath            string
	LogFollower        *follower.Follower
	MetricsRegistry    metrics.Registry
	PrometheusRegistry *prometheus.Registry
	PrometheusConfig   *prometheusmetrics.PrometheusConfig
	DebugEnabled       bool
}

type prometheusConfig struct {
	namespace     string
	Registry      metrics.Registry // Registry to be exported
	subsystem     string
	promRegistry  prometheus.Registerer //Prometheus registry
	FlushInterval time.Duration         //interval to update prom metrics
	gauges        map[string]prometheus.Gauge
}

func NewApp() *App {
	return &App{
		Applications: []Application{},
	}
}

func NewApplication(app *App, id int, applicationName string) Application {
	application := Application{
		ID:                id,
		ApplicationName:   applicationName,
		TotalLinesRead:    0,
		ReadRate:          0,
		LogTimeDifference: "",
	}
	return application
}
func (app *App) GetApplication(i int) *Application {
	app.Lock()
	defer app.Unlock()
	return &app.Applications[i] //CHANGED
}

func (app *App) AddApplication(id int, applicationName string, logPath string, maxRate int, debugEnabled bool) Application {
	app.Lock()
	defer app.Unlock()

	application := NewApplication(app, id, applicationName)
	application.ID = id
	application.LogFollower = application.createFollower(logPath)
	application.MetricsRegistry = application.createRegistry(applicationName)
	application.PrometheusConfig = prometheusmetrics.NewPrometheusProvider(application.MetricsRegistry, applicationName, "subsys", prometheus.DefaultRegisterer, 1*time.Second)
	application.DebugEnabled = debugEnabled
	go application.queueWorker(application.LogFollower, maxRate)

	if application.DebugEnabled == true {
		loggingInterval := 60 * time.Second
		application.enableLogging(applicationName, application.MetricsRegistry, loggingInterval)
	}
	return application
}

func (app *App) writeDebugMessage(debug bool, message string, applicationName string) {
	if debug == true {
		debugline := fmt.Sprintf("     %s ---------> %s", applicationName, message)
		fmt.Println(debugline)
	}
}

func (application *Application) createFollower(logPath string) *follower.Follower {
	logFollower, err := follower.New(logPath, follower.Config{
		Whence: io.SeekEnd,
		Offset: 0,
		Reopen: true,
	})
	fmt.Println(fmt.Sprintf("Attaching to: %s", logPath))
	if logFollower.Err() != nil {
		log.Println(logFollower.Err())
	}
	if err != nil {
		log.Println(err)
	}
	return logFollower
}

func (application *Application) queueWorker(alLog *follower.Follower, maxRate int) {
	meter := metrics.GetOrRegisterCounter("apm-log-read-rate", application.MetricsRegistry)
	//count := 0
	rl := ratelimit.New(maxRate) // per second
	for line := range alLog.Lines() {
		//use rate limiter
		rl.Take()

		application.CategorizeLogData(line.String(), application.ApplicationName, &application.MetricsRegistry, application.DebugEnabled)

		meter.Inc(1)
		application.TotalLinesRead++
	}
}

func (application *Application) createRegistry(logPath string) metrics.Registry {
	registry := metrics.NewRegistry()
	return registry
}

func (application *Application) enableLogging(applicationName string, registry metrics.Registry, intervalSec time.Duration) {
	/*         Metric Logging                                          */
	/*  uncomment to log metrics to a file applicationName.metrics.log */
	/*  for debugging purposes
	filename := fmt.Sprintf("%s.metrics.log", applicationName)
	logfile, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	//defer logfile.Close()

	log.SetOutput(logfile)
	go metrics.Log(registry, intervalSec, log.New(logfile, applicationName, log.Lmicroseconds))
	*/
}
