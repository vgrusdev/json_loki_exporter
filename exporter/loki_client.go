package exporter

import (
	"errors"
	"log/slog"
	"time"

	//"regexp"
	//"context"
	//"net"
	//"net/http"
	//"crypto/tls"
	//"strings"

	//"github.com/hooklift/gowsdl/soap"
	//"github.com/spf13/viper"
	//"github.com/vgrusdev/sap_system_exporter/internal/config"

	"github.com/vgrusdev/json_loki_exporter/config"

	"github.com/vgrusdev/promtail-client/promtail"
	//log "github.com/sirupsen/logrus"
)

func NewLokiClient(m config.Module) (promtail.Client, error) {

	//v := myConfig.Viper
	//log := config.NewLogger("sapcontrol-loki")
	//log.SetLevel(v.GetString("log_level"))

	lokiConfig := m.LokiClientConfig

	lokiURL := lokiConfig.URL
	if lokiURL == "" {
		return nil, nil
		//return nil, errors.New("Loki target URL is empty.")
	}
	loc, err := time.LoadLocation(lokiConfig.TimeLocation)
	if err != nil {
		loc, _ = time.LoadLocation("")
	}

	cfg := promtail.ClientConfig{
		Name:               lokiConfig.Name,
		PushURL:            lokiConfig.URL,
		TenantID:           lokiConfig.TenantId,
		BatchWait:          lokiConfig.BatchWait,
		BatchEntriesNumber: m.LokiClientConfig.BatchEntriesNumber,
		//Timeout:            time.Duration(timeout) * time.Millisecond,
		Timeout:  lokiConfig.LokiHttpTimeout,
		Location: loc,
	}

	//c, err := promtail.NewClientProto(&cfg)
	c, err := promtail.NewClientJson(&cfg)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// removes any duplicates in the array of comparable elements, e.g. structs
//
//	parameter - array, returns same type array, but w/o duplicated elements
func RemoveDuplicate[T comparable](sliceList []T) []T {
	allKeys := make(map[T]bool)
	list := []T{}
	for _, item := range sliceList {
		if _, value := allKeys[item]; !value {
			allKeys[item] = true
			list = append(list, item)
		}
	}
	return list
}

func getTimestamp(logger *slog.Logger, m JSONMetric, data []byte) time.Time {
	if m.EpochTimestampJSONPath == "" {
		return time.Now()
	}
	ts, err := extractValue(logger, data, m.EpochTimestampJSONPath, false)
	if err != nil {
		logger.Error("Failed to extract timestamp for metric", "path", m.KeyJSONPath, "err", err, "metric", m.Desc)
		return time.Now()
	}
	epochTime, err := SanitizeIntValue(ts)
	if err != nil {
		logger.Error("Failed to parse timestamp for metric", "path", m.KeyJSONPath, "err", err, "metric", m.Desc)
		return time.Now()
	}
	timestamp := time.UnixMilli(epochTime)
	return timestamp
}

func labelSetFromArrays(keys []string, values []string) (map[string]string, error) {
	m := make(map[string]string)
	if len(keys) != len(values) {
		return m, errors.New("labelSetFromArrays: Arrays should be the same length")
	}
	for i, key := range keys {
		m[key] = values[i]
	}
	return m, nil
}

func ratingToSeverity(r string) string {
	switch r {
	case "0":
		return "trace"
	case "1":
		return "info"
	case "2":
		return "warning"
	case "3":
		return "error"
	case "4":
		return "fatal"
	case "5":
		return "panic"
	default:
		return "unknown"
	}

}
