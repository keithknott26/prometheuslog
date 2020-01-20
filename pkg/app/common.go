package prometheuslog

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/rcrowley/go-metrics"
)

type Values struct {
	Value int64 `json:""`
}
type Interval struct {
	Begin string `json:"begin"`
	End   string `json:"end"`
}

type Datasource struct {
	UID_L3 string `json:"UID_L3"`
	UID_L4 string `json:"UID_L4"`
	Class  string `json:"Class"`
	Tag    string `json:"Tag"`
}

type Metrics struct {
	ApmAlertsRunning     *Values `json:"apmAlertsRunning,omitempty"`
	ApmAlertCreationRate *Values `json:"apmAlertCreationRate,omitempty"`
	PayloadReceptionRate *Values `json:"payloadReceptionRate,omitempty"`
	CaseCreationRate     *Values `json:"caseCreationRate,omitempty"`
	CaseTerminateRate    *Values `json:"caseTerminateRate,omitempty"`
}
type Data struct {
	Interval   *Interval   `json:"interval"`
	Metrics    *Metrics    `json:"metrics"`
	Datasource *Datasource `json:"dataSource"`
}

func (dashBoard *App) CategorizeLogData(line string, applicationName string, registry *metrics.Registry, debug bool) {
	/* This section is responsible for processing the logs. */
	/* Logs are read in line by line, the functions below   */
	/* are executed once per log line. It is recommended    */
	/* you try and stick to the more performant versions of */
	/* string matching like strings.Contains and run regex  */
	/* only when needed.                                    */

	//Parse memory usage statistics only when the memoryUsageIs log line is seen.
	if strings.Contains(line, "memoryUsageIs") {
		dashBoard.parseMemoryMessages(line, applicationName, *registry, debug)
	}
	//Parse common application log lines
	dashBoard.parseCommonMessages(line, applicationName, *registry, debug)

	//Parse JSON metrics emitted by the application
	if strings.Contains(line, "jsonMetricsMessageToBeSent") {
		dashBoard.parseMetricMessages(line, applicationName, *registry, debug)
	}

	//WARNs
	if strings.Contains(line, "WARN") {
		debugline := fmt.Sprintf("%s\n", line)
		dashBoard.writeDebugMessage(debug, debugline, applicationName)

		counter := metrics.GetOrRegisterCounter("common-warn-messages-total", *registry)
		counter.Inc(1)
	}
	//ERRORs
	if strings.Contains(line, "ERROR") {
		debugline := fmt.Sprintf("%s\n", line)
		dashBoard.writeDebugMessage(debug, debugline, applicationName)

		counter := metrics.GetOrRegisterCounter("common-error-messages-total", *registry)
		counter.Inc(1)
	}
	//FATALs
	if strings.Contains(line, "FATAL") {
		debugline := fmt.Sprintf("%s\n", line)
		dashBoard.writeDebugMessage(debug, debugline, applicationName)
		counter := metrics.GetOrRegisterCounter("apm-common-fatal-messages-total", *registry)
		counter.Inc(1)

	}

}
func (dashBoard *App) parseCommonMessages(line string, applicationName string, registry metrics.Registry, debug bool) {

	/*
		if strings.Contains(line, "postPayloadStarted") {
			dashBoard.writeDebugMessage(debug, "Common - Alert Created", applicationName)
			counter := metrics.GetOrRegisterCounter("apm-alert-created-total", registry)
			counter.Inc(1)
		}
		If log contains postPayloadStarted, write "Common - Alert Created" as the debug message
		and increment the <application name>_apm_alert_created_total metric by 1.
	*/
	if strings.Contains(line, "postPayloadStarted") {
		dashBoard.writeDebugMessage(debug, "Common - Alert Created", applicationName)
		counter := metrics.GetOrRegisterCounter("apm-alert-created-total", registry)
		counter.Inc(1)
	}

	/*  Float64 Gauge Example with regex capture group to grap the time in milliseconds
	log line looks like this:
	2019-12-28 00:44:45,714 [SocketListener1-25] DEBUG Scraper - scrapeExecuteFinished: completed=true,duration=769ms
	*/
	if strings.Contains(line, "scrapeExecuteFinished") {
		regex := regexp.MustCompile("scrapeExecuteFinished.*completed=true,duration=([0-9]{1,})ms")
		submatch := regex.FindStringSubmatch(line)
		if submatch != nil {
			duration := submatch[1]
			debugline := fmt.Sprintf("APM >> WebHarvest Thread Exit Duration: %v", duration)
			dashBoard.writeDebugMessage(debug, debugline, applicationName)

			s, _ := strconv.ParseFloat(duration, 64)
			meter := metrics.GetOrRegisterGaugeFloat64("apm-webharvest-exit-duration", registry)
			meter.Update(s)
		}
	}

}

func (dashBoard *App) parseMemoryMessages(line string, applicationName string, registry metrics.Registry, debug bool) {
	/* Parse Memory Messages
	   log line looks like this
	   2019.03.17 01:56:49,740 [Thread-252]  INFO com.impl.WatchdogProcessor - memoryUsageIs: used/free/total/max 187/192/380/455 Mb
	*/
	if strings.Contains(line, "memoryUsageIs") {
		regex := regexp.MustCompile("freeMemory=([0-9]{1,}) totalMemory=([0-9]{1,})")
		submatch := regex.FindStringSubmatch(line)
		if submatch != nil {
			freemem := submatch[1]
			totalmem := submatch[2]
			if debug == true {
				debugline := fmt.Sprintf("Common - Memory Report - Free %s Max: %s", freemem, totalmem)
				dashBoard.writeDebugMessage(debug, debugline, applicationName)
			}
			s, _ := strconv.ParseFloat(freemem, 64)
			meter := metrics.GetOrRegisterGaugeFloat64("apm-common-memoryfree-bytes", registry)
			meter.Update(s)

			s, _ = strconv.ParseFloat(totalmem, 64)
			meter = metrics.GetOrRegisterGaugeFloat64("apm-common-memorytotal-bytes", registry)
			meter.Update(s)
		}
	}

}

func (dashBoard *App) parseMetricMessages(line string, applicationName string, registry metrics.Registry, debug bool) {
	/*parse JSON Metrics from log
	log line looks like this (all on one line):
	`2019.11.30 02:45:00,007  INFO [com.metrics.collector.send.MetricsSenderAgent-Timer] com.impl.ALCore$Metrics - {"interval":{"end":"2019-11-30 02:45:00 +0000","begin":"2019-11-30 02:40:00 +0000"},"metrics":{"adlRefreshDuration":{"companyNameUSFRM":null},"fsEntryErrorRate":0,"adlTerminateRate":{"companyNameUSFRM":0},"fsCaseDeleteRate":0,"fsFilePickupRate":0,"adlAcceptDuration":{"companyNameUSFRM":null},"fsCaseUpdateRate":0,"adlRefreshRate":{"companyNameUSFRM":0},"adlResponseRate":{"companyNameUSFRM":1},"adlTerminateDuration":{"companyNameUSFRM":null},"adlDecisionInQueueSize":0,"adlInQueueSize":0,"adlLookupRate":{"companyNameUSFRM":1},"fsCasesActive":2514,"fsEntryReadRate":0,"fsFilePickupDuration":null,"adlOutQueueSize":0,"adlResponseDuration":{"companyNameUSFRM":null},"fsCaseCreateRate":0,"adlAlertCreationRate":{"companyNameUSFRM":0},"adlDecisionOutQueueSize":0,"adlLookupDuration":{"companyNameUSFRM":13},"fsInsertErrorRate":0,"fsCaseReplaceRate":0,"adlAlertsRunning":{"companyNameUSFRM":79}},"dataSource":{"UID_L3":"gibberfish","Class":"adeptraDecisionLink","Tag":"companyNameUSFRM"}}`
	*/

	//rescue is needed in case JSON parsing goes south
	defer rescue()

	if debug == true {
		debugline := fmt.Sprintf("Common - Processing Metrics")
		dashBoard.writeDebugMessage(debug, debugline, applicationName)
	}

	regex := regexp.MustCompile("^[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9].*(jsonMetricsMessageToBeSent)")
	match := regex.FindString(line)
	if match != "" {
		split := strings.Split(line, "json=")
		json_text := split[1]

		//unpack JSON into an interface map[string]interface{}
		var result map[string]interface{}
		json.Unmarshal([]byte(json_text), &result)
		metric := result["metrics"].(map[string]interface{})
		for key, _ := range metric {
			// Each value is an interface{} type, that is type asserted as a string
			//fmt.Println(metric[key], value)
			//spew.Dump(value)
			if key == "apmAlertsRunning" {
				val := fmt.Sprintf("%v", metric[key].(map[string]interface{})[applicationName])
				meter := metrics.GetOrRegisterGauge("apm-metric-alertsrunning-total", registry)
				valInt, err := strconv.ParseInt(val, 10, 64)
				if err == nil {
					meter.Update(valInt)
				}
			}
			if key == "apmAlertCreationRate" {
				val := fmt.Sprintf("%v", metric[key].(map[string]interface{})[applicationName])
				meter := metrics.GetOrRegisterGauge("apm-metric-alertcreationrate-total", registry)
				valInt, err := strconv.ParseInt(val, 10, 64)
				if err == nil {
					meter.Update(valInt)
				}
			}
			if key == "payloadReceptionRate" {
				val := fmt.Sprintf("%v", metric[key].(map[string]interface{})[applicationName])
				meter := metrics.GetOrRegisterGauge("apm-metric-payloadreceptionrate-total", registry)
				valInt, err := strconv.ParseInt(val, 10, 64)
				if err == nil {
					meter.Update(valInt)
				}
			}
			if key == "caseCreationRate" {
				val := fmt.Sprintf("%v", metric[key].(map[string]interface{})[applicationName])
				meter := metrics.GetOrRegisterGauge("apm-metric-casecreationrate-total", registry)
				valInt, err := strconv.ParseInt(val, 10, 64)
				if err == nil {
					meter.Update(valInt)
				}
			}
			if key == "caseTerminateRate" {
				val := fmt.Sprintf("%v", metric[key].(map[string]interface{})[applicationName])
				meter := metrics.GetOrRegisterGauge("apm-metric-caseterminaterate-total", registry)
				valInt, err := strconv.ParseInt(val, 10, 64)
				if err == nil {
					meter.Update(valInt)
				}
			}
		}

	}

}

func rescue() {
	// Check the execution state
	r := recover()
	// nil means we are in a normal execution
	if r != nil {
		// here, a panic has been occurred somewhere
		fmt.Printf("Panic (code %d) has been recovered from somewhere\n", r)
		// do something here to handle the panic
	}
}
