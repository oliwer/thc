/*
Package thc is a thin wrapper around Go's http.Client package witch provides:

 Metrics

THC exports metrics of your requests using expvar. You can observe average times for DNS lookups,
TLS handshakes, TCP sessions and more.

 Circuit breaker

After a defined number of consecutive failures, THC will switch to an *out of service* state.
In this state, the client will stop sending HTTP requests and instead will return the error
ErrOutOfService. It is up to the application to decide what to do in that case. After a
predefined amount of time, the service will be restores and THC will resume to work normally.
*/
package thc

import (
	"errors"
	"expvar"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/paulbellamy/ratecounter"
)

// ErrOutOfService is returned by the client when the maximum number
// of consecutive errors (MaxErrors) has been attained, and no HTTP
// request has been performed.
var ErrOutOfService = errors.New("HTTP client out of service")

const defaultHealingTime = 10 * time.Second

// THC - Timed HTTP Client. Implements the same interface as Go's http.Client.
type THC struct {
	// The HTTP client to use. Defaults to Go's HTTP client.
	Client *http.Client
	// Name is the prefix used for publishing expvars. Default: "thc".
	Name string
	// Number of errors after which the client becomes out of service.
	// Zero means never. Default: 0.
	MaxErrors int32
	// Lifespan of the out of service state. No HTTP requests are performed
	// in this state. Default: 10s.
	HealingTime time.Duration

	errorCounter int32
	metrics      Metrics
}

// Do sends an HTTP request and returns an HTTP response, following policy
// (such as redirects, cookies, auth) as configured on the client.
func (c *THC) Do(req *http.Request) (*http.Response, error) {
	if c.MaxErrors > 0 && atomic.LoadInt32(&c.errorCounter) >= c.MaxErrors {
		return nil, ErrOutOfService
	}

	// Set defaults.
	if c.Client == nil {
		c.Client = http.DefaultClient
	}
	if c.HealingTime == 0 {
		c.HealingTime = defaultHealingTime
	}
	if c.metrics.DNSLookup == nil {
		// User forgot to call PublishExpvar()
		c.PublishExpvar()
	}

	ctx := withTracing(req.Context(), &c.metrics)
	req = req.WithContext(ctx)

	res, err := c.Client.Do(req)

	if c.MaxErrors > 0 {
		if err != nil || res.StatusCode >= 500 {
			// Become out of service if we have reached MaxErrors
			if atomic.AddInt32(&c.errorCounter, 1) == c.MaxErrors {
				c.metrics.OutOfService.Incr(1)
				// Restore the service after some time.
				go func() {
					time.Sleep(c.HealingTime)
					atomic.StoreInt32(&c.errorCounter, 0)
				}()
			}
		} else {
			// No error. Reset the counter to zero.
			atomic.StoreInt32(&c.errorCounter, 0)
		}
	}

	return res, err
}

// Get issues a GET to the specified URL.
func (c *THC) Get(url string) (resp *http.Response, err error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	return c.Do(req)
}

// Head issues a HEAD to the specified URL.
func (c *THC) Head(url string) (resp *http.Response, err error) {
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return nil, err
	}

	return c.Do(req)
}

// Post issues a POST to the specified URL.
func (c *THC) Post(url string, contentType string, body io.Reader) (resp *http.Response, err error) {
	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", contentType)

	return c.Do(req)
}

// PostForm issues a POST to the specified URL,
// with data's keys and values URL-encoded as the request body.
func (c *THC) PostForm(url string, data url.Values) (resp *http.Response, err error) {
	return c.Post(url, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
}

// PublishExpvar will publish all the metrics for a THC instance.
// This method should be called from the `init` function in your program.`
// The metrics' names are prefixed with the Name specified in the THC object.
// Exported metrics:
//   <name>-dns-lookup
//   <name>-tcp-connection
//   <name>-tls-handshake
//   <name>-get-connection
//   <name>-write-request
//   <name>-get-response
//   <name>-outofservice
func (c *THC) PublishExpvar() {
	const rate = 1 * time.Minute

	n := c.Name
	if n == "" {
		n = "thc"
	}

	c.metrics.DNSLookup = ratecounter.NewAvgRateCounter(rate)
	expvar.Publish(n+"-dns-lookup", c.metrics.DNSLookup)

	c.metrics.TCPConnection = ratecounter.NewAvgRateCounter(rate)
	expvar.Publish(n+"-tcp-connection", c.metrics.TCPConnection)

	c.metrics.TLSHandshake = ratecounter.NewAvgRateCounter(rate)
	expvar.Publish(n+"-tls-handshake", c.metrics.TLSHandshake)

	c.metrics.GetConnection = ratecounter.NewAvgRateCounter(rate)
	expvar.Publish(n+"-get-connection", c.metrics.GetConnection)

	c.metrics.WriteRequest = ratecounter.NewAvgRateCounter(rate)
	expvar.Publish(n+"-write-request", c.metrics.WriteRequest)

	c.metrics.GetResponse = ratecounter.NewAvgRateCounter(rate)
	expvar.Publish(n+"-get-response", c.metrics.GetResponse)

	c.metrics.OutOfService = ratecounter.NewRateCounter(1 * time.Hour)
	expvar.Publish(n+"-outofservice", c.metrics.OutOfService)
}
