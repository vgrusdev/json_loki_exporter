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

package exporter

import (
	"bytes"
	"encoding/json"
	"strings"

	//"errors"
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/vgrusdev/json_loki_exporter/config"
	"github.com/vgrusdev/promtail-client/promtail"
	"k8s.io/client-go/util/jsonpath"
)

type JSONMetricCollector struct {
	JSONMetrics []JSONMetric
	Data        []byte // result of request to target by FetchJSON(endpoint string)
	Logger      *slog.Logger
	LokiClient  promtail.Client
}

type JSONMetric struct {
	Desc                   *prometheus.Desc
	Type                   config.ScrapeType    // string: "value", "object", "loki"
	KeyJSONPath            string               // root path for patterm
	ValueJSONPath          string               // path for Value
	LabelsJSONPaths        []string             // path for labels (labael names are in Desc.variableLabels)
	LabelsNames            []string             // Names of Labels, assigned in util.CreateMetricsList()
	ValueType              prometheus.ValueType // "gauge", "counter" etc.
	EpochTimestampJSONPath string               // path for timestamp
}

func (mc JSONMetricCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, m := range mc.JSONMetrics {
		switch m.Type {
		case config.ValueScrape:
			fallthrough
		case config.ObjectScrape:
			ch <- m.Desc
		}
	}
}

func (mc JSONMetricCollector) Collect(ch chan<- prometheus.Metric) {
	for _, m := range mc.JSONMetrics {
		mc.Logger.Debug("mc.JSONMetrics loop", "m", m)
		switch m.Type {
		case config.ValueScrape:
			value, err := extractValue(mc.Logger, mc.Data, m.KeyJSONPath, false)
			mc.Logger.Debug("mc.JSONMetrics loop, ValueScrape", "value", value)
			if err != nil {
				mc.Logger.Error("Failed to extract value for metric", "path", m.KeyJSONPath, "err", err, "metric", m.Desc)
				continue
			}

			if floatValue, err := SanitizeValue(value); err == nil {
				metric := prometheus.MustNewConstMetric(
					m.Desc,
					m.ValueType,
					floatValue,
					extractLabels(mc.Logger, mc.Data, m.LabelsJSONPaths)...,
				)
				ch <- timestampMetric(mc.Logger, m, mc.Data, metric)
			} else {
				mc.Logger.Error("Failed to convert extracted value to float64", "path", m.KeyJSONPath, "value", value, "err", err, "metric", m.Desc)
				continue
			}

		case config.ObjectScrape:
			values, err := extractValue(mc.Logger, mc.Data, m.KeyJSONPath, true)
			mc.Logger.Debug("mc.JSONMetrics loop, ObjectScrape", "values", values)
			if err != nil {
				mc.Logger.Error("Failed to extract json objects for metric", "err", err, "metric", m.Desc)
				continue
			}

			var jsonData []interface{}
			if err := json.Unmarshal([]byte(values), &jsonData); err == nil {
				mc.Logger.Debug("mc.JSONMetrics loop, ObjectScrape", "jsonData", jsonData)
				for _, data := range jsonData {
					mc.Logger.Debug("mc.JSONMetrics loop, ObjectScrape, jsonData", "data", data)
					jdata, err := json.Marshal(data)
					if err != nil {
						mc.Logger.Error("Failed to marshal data to json", "path", m.ValueJSONPath, "err", err, "metric", m.Desc, "data", data)
						continue
					}
					value, err := extractValue(mc.Logger, jdata, m.ValueJSONPath, false)
					mc.Logger.Debug("mc.JSONMetrics loop, ObjectScrape, jsonData", "value", value)
					if err != nil {
						mc.Logger.Error("Failed to extract value for metric", "path", m.ValueJSONPath, "err", err, "metric", m.Desc)
						continue
					}

					if floatValue, err := SanitizeValue(value); err == nil {
						metric := prometheus.MustNewConstMetric(
							m.Desc,
							m.ValueType,
							floatValue,
							extractLabels(mc.Logger, jdata, m.LabelsJSONPaths)...,
						)
						mc.Logger.Debug("mc.JSONMetrics loop, timestampMetric sending ", "metric", metric)
						ch <- timestampMetric(mc.Logger, m, jdata, metric)
					} else {
						mc.Logger.Error("Failed to convert extracted value to float64", "path", m.ValueJSONPath, "value", value, "err", err, "metric", m.Desc)
						continue
					}
				}
			} else {
				mc.Logger.Error("Failed to convert extracted objects to json", "err", err, "metric", m.Desc)
				continue
			}

			// ======================= vg ======================================

		case config.LokiScrape:
			if mc.LokiClient == nil {
				continue
			}
			values, err := extractValue(mc.Logger, mc.Data, m.KeyJSONPath, true)
			mc.Logger.Debug("mc.JSONMetrics loop, LokiScrape", "values", values)
			if err != nil {
				mc.Logger.Error("Failed to extract json objects for metric", "err", err, "metric", m.Desc)
				continue
			}

			var jsonData []interface{}
			/* VG+ */
			decoder := json.NewDecoder(strings.NewReader(values))
			decoder.UseNumber() // This keeps numbers as json.Number

			if err := decoder.Decode(&jsonData); err != nil {
				/* VG- */
				//if err := json.Unmarshal([]byte(values), &jsonData); err != nil {
				mc.Logger.Error("Failed to convert extracted objects to json", "err", err, "metric", m.Desc)
				continue
			}
			mc.Logger.Debug("mc.JSONMetrics loop, LokiScrape", "jsonData", jsonData)

			for _, data := range jsonData {
				mc.Logger.Debug("mc.JSONMetrics loop, LokiScrape, jsonData", "data", data)
				jdata, err := json.Marshal(data)
				if err != nil {
					mc.Logger.Error("Failed to marshal data to json", "path", m.ValueJSONPath, "err", err, "metric", m.Desc, "data", data)
					continue
				}
				value, err := extractValue(mc.Logger, jdata, m.ValueJSONPath, false) // value is an alert message here
				mc.Logger.Debug("mc.JSONMetrics loop, ObjectScrape, jsonData", "value", value)
				if err != nil {
					mc.Logger.Error("Failed to extract value for metric", "path", m.ValueJSONPath, "err", err, "metric", m.Desc)
					continue
				}

				labelSet, err := labelSetFromArrays(m.LabelsNames, extractLabels(mc.Logger, jdata, m.LabelsJSONPaths))
				if err != nil {
					mc.Logger.Warn("Alert metrics LabelSet mismatch", "err", err)
					continue
				}
				message := value
				timestamp := getTimestamp(mc.Logger, m, jdata)
				if rating, ok := labelSet["alert_rating"]; ok {
					labelSet["level"] = ratingToSeverity(rating)
				}

				sInputEntry := promtail.SingleEntry{
					Labels: labelSet,
					Ts:     timestamp,
					Line:   message,
				}
				mc.LokiClient.Single() <- &sInputEntry
			}
		default:
			mc.Logger.Error("Unknown scrape config type", "type", m.Type, "metric", m.Desc)
			continue
		}
	}
}

// Returns the last matching value at the given json path
func extractValue(logger *slog.Logger, data []byte, path string, enableJSONOutput bool) (string, error) {
	var jsonData interface{}
	buf := new(bytes.Buffer)

	j := jsonpath.New("jp")
	if enableJSONOutput {
		j.EnableJSONOutput(true)
	}
	/* VG+ */
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.UseNumber() // This keeps numbers as json.Number

	if err := decoder.Decode(&jsonData); err != nil {
		/* VG- */
		//if err := json.Unmarshal(data, &jsonData); err != nil {
		logger.Error("Failed to unmarshal data to json", "err", err, "data", data)
		return "", err
	}

	if err := j.Parse(path); err != nil {
		logger.Error("Failed to parse jsonpath", "err", err, "path", path, "data", data)
		return "", err
	}

	if err := j.Execute(buf, jsonData); err != nil {
		logger.Error("Failed to execute jsonpath", "err", err, "path", path, "data", data)
		return "", err
	}

	// Since we are finally going to extract only float64, unquote if necessary
	if res, err := jsonpath.UnquoteExtend(buf.String()); err == nil {
		return res, nil
	}

	return buf.String(), nil
}

// Returns the list of labels created from the list of provided json paths
func extractLabels(logger *slog.Logger, data []byte, paths []string) []string {
	labels := make([]string, len(paths))
	for i, path := range paths {
		if result, err := extractValue(logger, data, path, false); err == nil {
			labels[i] = result
		} else {
			logger.Error("Failed to extract label value", "err", err, "path", path, "data", data)
		}
	}
	return labels
}

func timestampMetric(logger *slog.Logger, m JSONMetric, data []byte, pm prometheus.Metric) prometheus.Metric {
	if m.EpochTimestampJSONPath == "" {
		return pm
	}
	ts, err := extractValue(logger, data, m.EpochTimestampJSONPath, false)
	if err != nil {
		logger.Error("Failed to extract timestamp for metric", "path", m.KeyJSONPath, "err", err, "metric", m.Desc)
		return pm
	}
	epochTime, err := SanitizeIntValue(ts)
	if err != nil {
		logger.Error("Failed to parse timestamp for metric", "path", m.KeyJSONPath, "err", err, "metric", m.Desc)
		return pm
	}
	timestamp := time.UnixMilli(epochTime)
	return prometheus.NewMetricWithTimestamp(timestamp, pm)
}
