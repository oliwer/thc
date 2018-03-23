package thc_test

import (
	"net/http"
	"time"

	"github.com/oliwer/thc"
)

var client = &thc.THC{
	Client:      &http.Client{Timeout: 10 * time.Millisecond},
	Name:        "example",
	MaxErrors:   10,
	HealingTime: 20 * time.Second,
}

func init() {
	client.PublishExpvar()
}

func ExampleTHC() {
	for {
		resp, err := client.Get("https://error500.nope/")

		if err == thc.ErrOutOfService {
			// The service is down for 20s. (HealingTime)
			break
		}

		if err != nil {
			// There was an error but we are still OK because
			// still under MaxErrors consecutive errors.
			continue
		}

		// Process resp normally...
		resp.Body.Close()
	}
}
