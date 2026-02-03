package exporter

import (
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

	c, err := promtail.NewClientProto(&cfg)
	//c, err := promtail.NewClientJson(&cfg)
	if err != nil {
		return nil, err
	}
	return c, nil
}
