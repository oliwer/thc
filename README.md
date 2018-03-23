THC - Timed HTTP Client for Go
==============================

[![GoDoc](https://godoc.org/github.com/oliwer/thc?status.svg)](https://godoc.org/github.com/oliwer/thc)
[![Go Report Card](https://goreportcard.com/badge/oliwer/thc)](https://goreportcard.com/report/oliwer/thc)

THC is a thin wrapper around Go's `http.Client` package witch provides these extra features.

## Metrics

THC exports metrics of your requests using `expvar`. You can observe average times for DNS lookups,
TLS handshakes, TCP sessions and more. Look at [the documentation](https://godoc.org/github.com/oliwer/thc)
for a list of exported metrics.

## Circuit breaker

After a defined number of consecutive failures, THC will switch to an *out of service* state.
In this state, the client will stop sending HTTP requests and instead will return the error
`ErrOutOfService`. It is up to the application to decide what to do in that case. After a
predefined amount of time, the service will be restores and THC will resume to work normally.

# Example

    package main

    import (
        "net/http"
        "time"

        "github.com/oliwer/thc"
    )

    var client = &thc.THC{
        Client:      &http.Client{Timeout: 100 * time.Millisecond},
        Name:        "example",
        MaxErrors:   10,
        HealingTime: 20 * time.Second,
    }

    func init() {
        client.PublishExpvar()
    }

    func main() {
        for {
            resp, err := client.Get("https://example.com/thing.json")

            if err == thc.ErrOutOfService {
                // The service is down for 20s. (HealingTime)
            }

            if err != nil {
                // There was an error but we are still OK because
                // still under MaxErrors consecutive errors.
            }

            // Process resp normally...
        }
    }

# Notes

THC requires Go v1.8 or above. It is thread-safe and lock-less.
