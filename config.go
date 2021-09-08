package main

import (
	"fmt"

	"github.com/kelseyhightower/envconfig"
	"toolman.org/numbers/stdsize"
)

type brokerConfig struct {
	BrokerUsername     string `envconfig:"broker_username" required:"true"`
	BrokerPassword     string `envconfig:"broker_password" required:"true"`
	OntapURL           string `envconfig:"ontap_url" required:"true"`
	OntapUser          string `envconfig:"ontap_user" required:"true"`
	OntapPassword      string `envconfig:"ontap_password" required:"true"`
	OntapSkipSSLCheck  bool   `envconfig:"ontap_skip_ssl_check" required:"true"`
	OntapSvmName       string `envconfig:"ontap_svm_name" required:"true"`
	CifsHostname       string `envconfig:"cifs_hostname" required:"true"`
	TrustedSSHKey      string `envconfig:"trusted_ssh_key" default:""`
	MaxVolumeSize      string `envonconfig:"max_volume_size" default:"2Ti"`
	MaxVolumeSizeBytes int64
	LogLevel           string `envconfig:"log_level" default:"INFO"`
	Port               string `envconfig:"port" default:"3000"`
	DocsURL            string `envconfig:"docsurl" default:"default"`
}

func brokerConfigLoad() (brokerConfig, error) {
	var config brokerConfig
	err := envconfig.Process("", &config)
	if err != nil {
		return brokerConfig{}, err
	}

	//convert MaxVolumeSize once. Also makes sure it's valid at broker start
	size, err := stdsize.Parse(config.MaxVolumeSize)
	if err != nil {
		return brokerConfig{}, fmt.Errorf("Unable to parse MAX_VOLUME_SIZE. Allowed modifiers: K,M,G,T,P,Ki,Mi,Gi,Ti,Pi")
	}

	if size < 20971520 {
		return brokerConfig{}, fmt.Errorf("MAX_VOLUME_SIZE too small")
	}

	config.MaxVolumeSizeBytes = int64(size)

	return config, nil
}
