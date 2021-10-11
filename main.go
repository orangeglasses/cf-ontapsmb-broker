package main

import (
	"fmt"
	"net/http"
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/pivotal-cf/brokerapi/v7"
)

func main() {
	var logLevels = map[string]lager.LogLevel{
		"DEBUG": lager.DEBUG,
		"INFO":  lager.INFO,
		"ERROR": lager.ERROR,
		"FATAL": lager.FATAL,
	}

	config, err := brokerConfigLoad()
	if err != nil {
		panic(err)
	}

	brokerCredentials := brokerapi.BrokerCredentials{
		Username: config.BrokerUsername,
		Password: config.BrokerPassword,
	}

	services, err := CatalogLoad("./catalog.json")
	if err != nil {
		panic(err)
	}

	for i := range services {
		services[i].Metadata.DocumentationUrl = config.DocsURL
	}

	logger := lager.NewLogger("cf-ontapsmb-broker")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, logLevels[config.LogLevel]))

	ontapClient, _ := NewOntapClient(config.OntapUser, config.OntapPassword, config.TrustedSSHKey, config.OntapURL, true)

	serviceBroker := &broker{
		services:    services,
		env:         config,
		ontapClient: ontapClient,
	}

	brokerHandler := brokerapi.New(serviceBroker, logger, brokerCredentials)
	fmt.Println("Starting service")
	http.Handle("/", brokerHandler)
	http.ListenAndServe(":"+config.Port, nil)
}
