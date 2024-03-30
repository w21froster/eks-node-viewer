package agent

import (
	"github.com/newrelic/go-agent/v3/integrations/nrlogrus"
	"github.com/newrelic/go-agent/v3/newrelic"
	log "github.com/sirupsen/logrus"
	"time"
)

//-----------------------------------------------------------------------------
// setupNewRelicApp
//-----------------------------------------------------------------------------

func SetupApp(appName, hostname, licenseKey string) (*newrelic.Application, error) {
	app, err := newrelic.NewApplication(
		newrelic.ConfigAppName(appName),
		newrelic.ConfigLicense(licenseKey),
		func(config *newrelic.Config) {
			log.SetLevel(log.InfoLevel)
			config.Logger = nrlogrus.StandardLogger()
			config.Host = hostname
		},
	)
	if err != nil {
		log.Error(err.Error())
		return nil, err
	}

	err = app.WaitForConnection(300 * time.Second)
	if err != nil {
		log.Error(err.Error())
		return nil, err
	}

	return app, nil
}
