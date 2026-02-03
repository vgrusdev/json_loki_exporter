// Copyright 2020 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"log/slog"
	"os"
	"time"

	pconfig "github.com/prometheus/common/config"
	"gopkg.in/yaml.v2"
)

// Metric contains values that define a metric
type Metric struct {
	Name           string            `yaml:"name"`
	Path           string            `yaml:"path"`
	Labels         map[string]string `yaml:"labels"`
	Type           ScrapeType        `yaml:"type"`
	ValueType      ValueType         `yaml:"valuetype"`
	EpochTimestamp string            `yaml:"epochTimestamp"`
	Help           string            `yaml:"help"`
	Values         map[string]string `yaml:"values"`
	//Messages       map[string]string
}

type ScrapeType string

const (
	ValueScrape  ScrapeType = "value" // default
	ObjectScrape ScrapeType = "object"
	LokiScrape   ScrapeType = "loki"
)

type ValueType string

const (
	ValueTypeGauge   ValueType = "gauge"
	ValueTypeCounter ValueType = "counter"
	ValueTypeUntyped ValueType = "untyped"
)

// Config contains multiple modules.
type Config struct {
	Modules map[string]Module `yaml:"modules"`
}

// Module contains metrics and headers defining a configuration
type Module struct {
	Headers          map[string]string        `yaml:"headers,omitempty"`
	Metrics          []Metric                 `yaml:"metrics"`
	HTTPClientConfig pconfig.HTTPClientConfig `yaml:"http_client_config,omitempty"`
	Body             Body                     `yaml:"body,omitempty"`
	ValidStatusCodes []int                    `yaml:"valid_status_codes,omitempty"`
	LokiClientConfig LokiConfig               `yaml:"loki_client_config,omitempty"`
}

type Body struct {
	Content    string `yaml:"content"`
	Templatize bool   `yaml:"templatize,omitempty"`
}

type LokiConfig struct {
	// URL - full url_path to push LOKI stream, e.g. http://localhost:3100/loki/api/v1/push, if empty srtring, LOKI will not be PUSHED
	URL string `yaml:"loki_url"`

	// "name" label will be added to all other labers, so LOKI will recognise it as service_name
	Name string `yaml:"loki_name"`

	// TenantId is used to separate different loki streams. if not set "fake" TenantID will be set.
	TenantId string `yaml:"loki_tenant_id,omitempty"`

	// valid duration time units are: ms, s, m, h
	AlertSamplesMaxAge time.Duration `yaml:"alert_samples_max_age,omitempty"`

	// Max wait time before bunch of messages will be sent to LOKI
	BatchWait time.Duration `yaml:"loki_batch_wait,omitempty"`

	// batch will be pushed to LOKI in case number of messahes exceeds loki_batch_entries_number or loki_batch_wait will expire
	BatchEntriesNumber int `yaml:"loki_batch_entries_number,omitempty"`

	// HTTP POST timeout in case LOKI server does not response
	LokiHttpTimeout time.Duration `yaml:"loki_http_timeout,omitempty"`

	// Alert time Location
	TimeLocation string `yaml:"loki_time_location,omitempty"`

	// Send alert metrics to prom in additional to Loki
	SendAlertsToProm bool `yaml:"send_alerts_to_prom,omitempty"`

	/*
	   send_alerts_to_prom: false
	   # valid duration time units are: ms, s, m, h
	   alert_samples_max_age: "2h"
	   # Loki section.
	   # sap-alerts will be written to the LOKI server in case of loki_url is not empty string.
	   #
	   # loki_url - full url_path to push LOKI stream, e.g. http://localhost:3100/loki/api/v1/push
	   # if empty srtring, LOKI will not be PUSHED
	   loki_url: "http://sapgraf.moscow.alfaintra.net:3100/loki/api/v1/push"
	   #
	   # loki_name - "name" label will be added to all other labers, so LOKI will recognise it as servce_name
	   #loki_name: "sap_alerts"
	   loki_name: "sap_alerts"
	   #
	   # LOKI Tenant ID: used to separate different loki streams. if not set fake TenantID will be set.
	   loki_tenantid: "sap_alerts"
	   #
	   # loki_batch_wait - Max wait time (Milliseconds) before bunch of messages will be sent to LOKI
	   loki_batch_wait: "100ms"
	   #
	   # loki_batch_entries_number - Size of the buch buffer.
	   # batch will be pushed to LOKI in case number of messahes exceed loki_batch_entries_number or loki_batch_wait will expire
	   loki_batch_entries_number: 32
	   #
	   # loki_http_timeout - HTTP POST timeout in case LOKI server does not response
	   loki_http_timeout: "1000ms"
	   #
	   # loki_time_location - Alert time Location
	   loki_time_location: "Europe/Moscow"
	*/

}

func LoadConfig(logger *slog.Logger, configPath string) (Config, error) {
	var config Config
	data, err := os.ReadFile(configPath)
	if err != nil {
		return config, err
	}

	logger.Debug("Loaded config file data", "config", string(data))

	if err := yaml.Unmarshal(data, &config); err != nil {
		return config, err
	}

	logger.Debug("Loaded config file struct", "config", config)

	// Complete Defaults
	for name, module := range config.Modules {
		for i := 0; i < len(module.Metrics); i++ {
			if module.Metrics[i].Type == "" {
				module.Metrics[i].Type = ValueScrape
			}
			if module.Metrics[i].Help == "" {
				module.Metrics[i].Help = module.Metrics[i].Name
			}
			if module.Metrics[i].ValueType == "" {
				module.Metrics[i].ValueType = ValueTypeUntyped
			}
		}
		loki := module.LokiClientConfig
		if len(loki.URL) > 0 {
			if loki.Name == "" {
				logger.Warn("config for module " + name + " has empty value for loki_name !")
			}
			if loki.TenantId == "" {
				logger.Warn("config for module " + name + " has empty value for loki_tenant_id. Will use \"fake\"")
				loki.TenantId = "fake"
			}
			if loki.AlertSamplesMaxAge <= 0*time.Second {
				loki.AlertSamplesMaxAge = 2 * time.Hour
			}
			if loki.BatchEntriesNumber <= 0 {
				loki.BatchEntriesNumber = 1
			}
			if loki.BatchWait <= 0*time.Millisecond {
				loki.BatchWait = 100 * time.Millisecond
			}
			if loki.LokiHttpTimeout <= 0*time.Millisecond {
				loki.LokiHttpTimeout = 10 * time.Second
			}
			if _, err := time.LoadLocation(loki.TimeLocation); err != nil {
				logger.Warn("config for module "+name+" has incorrect time_location, use UTC.", "loki_time_location", loki.TimeLocation, "err", err)
				loki.TimeLocation = ""
			}
		} else {
			logger.Warn("config for module " + name + " has empty value for loki_url. Data will not be sent to Loki")
		}
	}
	return config, nil
}
