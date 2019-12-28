# prometheuslog
Tail Multiple log files at once, expose metrics about those log files to promtheus.

## Description
This app represents an easy way to monitor (tail) multiple application log files and expose a /metrics endpoint with stats about that same log file. The metrics exposed are in prometheus format so they can easily be scraped and imported into grafana.

* Features
  * Parse different types of logs (plaintext or JSON)
  * Automatically re-open log file in the event of log rollover
  * Specify infinite amount of logs to monitor
  * Built in rate-limiter to give processing priority to other applications
  * Read config from configuration file
  * Specify application / log file name (this name will be used in metric exposed)
  * Configurable listening port
  * Configurable environment (specify 'staging', 'uat', 'prod'). This identifier is required and is used to formulate the metric name.
  * Configurable metrics flush interval
  * Log metrics to disk (log file) when debug is enabled

## Instructions for use
This application will need to be modified before it will work for you, it was designed to be a starting point only. The configuration to edit/add to will be in common.go where you specify both the log line you'd like to search for and the metric name you'd like to keep track of the value in. You'll build upon common.go with additional functions or conditions.

**Please note that dashes in metric names are converted to underscores automatically. Metric name "apm-alert-created-total" in the code becomes "apm_alert_created_total" when its exposed to the /metrics endpoint.

For Example:
```
if strings.Contains(line, "postPayloadStarted") {
    dashBoard.writeDebugMessage(debug, "Common - Alert Created", applicationName)
    counter := metrics.GetOrRegisterCounter("apm-alert-created-total", registry)
    counter.Inc(1)
}
// If log contains postPayloadStarted, write "Common - Alert Created" as the debug message
// and increment the <application name>_apm_alert_created_total metric by 1.
```

Note: Because the functions in common.go get executed once per log line, you'll want to use performant string matching such as strings.Contains or regex only when necessary.

***This app has been load tested up to 100k operations per/sec using strings.Contains.

Remember to populate the config file, and specify it with the -c argument when starting the app. The format for the configuration file is as follows:

### Config File (prometheuslog.conf)
```
myFirstApplication,/Users/myuser/filename-1.log
mySecondApplication,/Users/myuser/filename-2.log
myThirdApplication,/Users/myuser/filename-3.log
```

Once the app is up and running, a /metrics endpoint will be populated on a port (default: 9091) which should contain stats about the log (assuming there were string matches found). You can then poll the metrics endpoint from a browser or use curl: curl -X http://localhost:9091/metrics

### Prometheus Scrape Configuration
Once you see that your metrics are populated and changing, you can configure prometheus.  I used the following scraping config which assumes the following metrics format:   <applicationname>_<environment>_<metricname>

```
 - job_name: 'myJobName'
   scrape_interval: 10s
   scrape_timeout: 10s
   honor_labels: false
   static_configs:
     - targets: ['localhost:9091']
   metric_relabel_configs:
    - source_labels: ['__name__']
      regex: "(\\w+(?:_\\w+|))_(prod|uat|staging)_.*"
      target_label: "app"
      replacement: "$1"
    - source_labels: ["__name__"]
      regex: "(\\w+(_\\w+|))_(prod|uat|staging)_.*"
      target_label: "environment"
      replacement: "$3"
    - source_labels: ["__name__"]
      regex: "(?:\\w+(?:_\\w+|))_(?:prod|uat|staging)_(.*)"
      target_label: "__name__"
      replacement: "$1"
```

## Installation
```bash
$ go get -u github.com/keithknott26/prometheuslog
```
### Building
```bash
$ cd github.com/keithknott26/prometheuslog/cmd/;
$ go build prometheuslog.go
```
### Arguments
```bash
usage: prometheuslog [<flags>]

Expose Metrics endpoint to Prometheus/Grafana

Flags:
      --help                     Show context-sensitive help (also try --help-long and --help-man).
      --debug                    Enable Debug Mode
  -p, --port=9091                Port to listen for metrics requests. Default: 9091
  -e, --environment="prod"       Environment (staging, uat, or prod). Default: prod
  -f, --flush-interval=2s        How often to flush available metrics: (1s,5s,15s,1h,etc) (default: 2s) ...)
  -r, --max-ingestion-rate=10000  
                                 Ingestion Rate Limiter:(1000,5000,10000,etc) in operations per/sec (default: 10000) ...)
  -c, --config-file=CONFIG-FILE  Full path to the prometheuslog.conf config file.

Args:
  None
```

### License
MIT