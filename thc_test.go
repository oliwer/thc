package thc

import (
	"expvar"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"
)

func TestMetrics(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(20 * time.Millisecond)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	// Default settings, no circuit breaker.
	client := THC{}
	client.PublishExpvar()

	// Helpers
	r := func(resp *http.Response, err error) error {
		resp.Body.Close()
		return err
	}
	c := func(t *testing.T, err error) {
		if err != nil {
			t.Error(err)
		}
	}

	// Do a few requests.
	c(t, r(client.Get(server.URL)))
	c(t, r(client.Head(server.URL)))
	c(t, r(client.Post(server.URL, "text/plain", nil)))
	c(t, r(client.PostForm(server.URL, url.Values{})))

	assert := func(key string, val float64, cond bool) {
		if !cond {
			t.Error(key, val)
		}
	}

	var t1, t2, t3 float64

	// Read expvars in random order and assert the values are coherent.
	expvar.Do(func(elem expvar.KeyValue) {
		k := elem.Key
		v, _ := strconv.ParseFloat(elem.Value.String(), 64)

		switch k {
		case "thc-dns-lookup":
			assert(k, v, v == 0) // We connect to an IP address.
		case "thc-tcp-connection":
			assert(k, v, v > 0)
			t1 = v
		case "thc-tls-handshake":
			assert(k, v, v == 0) // No TLS in this test.
		case "thc-get-connection":
			assert(k, v, v > t1)
			t2 = v
		case "thc-write-request":
			assert(k, v, v > t2)
			t3 = v
		case "thc-get-response":
			assert(k, v, v > t3)
		}
	})
}

func TestCircuitBreaker(t *testing.T) {
	const (
		maxErrors   = 2
		healingTime = 100 * time.Millisecond
	)

	// A server which always fails.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "FAIL", 500)
	}))
	defer server.Close()

	// A client that becomes out of service after 2 failures, for 100ms.
	client := THC{
		Name:        "test",
		MaxErrors:   maxErrors,
		HealingTime: healingTime,
	}

	// These first requests should be fine.
	for i := 0; i < maxErrors; i++ {
		if _, err := client.Get(server.URL); err != nil {
			t.Error("unexpected error:", err)
		}
	}

	// The next request should fail.
	if _, err := client.Get(server.URL); err != ErrOutOfService {
		t.Error("expected OutOfService error")
	}

	time.Sleep(healingTime + 1*time.Millisecond)

	// The client should be back in service now.
	if _, err := client.Get(server.URL); err != nil {
		t.Error("unexpected error:", err)
	}
}
