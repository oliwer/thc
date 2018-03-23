package thc

import (
	"context"
	"crypto/tls"
	"net/http/httptrace"
	"time"

	"github.com/paulbellamy/ratecounter"
)

// Metrics exported via expvar. The times are in nanoseconds and are
// averages per minute.
type Metrics struct {
	// Time to perform a DNS Lookup.
	DNSLookup *ratecounter.AvgRateCounter
	// Time to open a new TCP connection.
	TCPConnection *ratecounter.AvgRateCounter
	// Time to perform a TLS handshake.
	TLSHandshake *ratecounter.AvgRateCounter

	// Total time to get a ready-to-use connection. This includes the
	// previous 3 metrics. It may be zero if you use a connection pool.
	GetConnection *ratecounter.AvgRateCounter
	// Total time taken to send the HTTP request, including the time
	// to get a connection.
	WriteRequest *ratecounter.AvgRateCounter
	// Total time elapsed until we got the first byte of the response.
	// This includes the previous metric.
	GetResponse *ratecounter.AvgRateCounter

	// Counter indicating how many times per hour was the client out of service.
	OutOfService *ratecounter.RateCounter
}

func withTracing(ctx context.Context, m *Metrics) context.Context {
	var t0, dnsStart, tcpStart, tlsStart time.Time

	return httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{
		GetConn: func(_ string) { t0 = time.Now() },

		DNSStart: func(_ httptrace.DNSStartInfo) { dnsStart = time.Now() },
		DNSDone: func(_ httptrace.DNSDoneInfo) {
			m.DNSLookup.Incr(time.Now().Sub(dnsStart).Nanoseconds())
		},

		ConnectStart: func(_, _ string) { tcpStart = time.Now() },
		ConnectDone: func(_, _ string, _ error) {
			m.TCPConnection.Incr(time.Now().Sub(tcpStart).Nanoseconds())
		},

		TLSHandshakeStart: func() { tlsStart = time.Now() },
		TLSHandshakeDone: func(_ tls.ConnectionState, _ error) {
			m.TLSHandshake.Incr(time.Now().Sub(tlsStart).Nanoseconds())
		},

		GotConn: func(_ httptrace.GotConnInfo) {
			m.GetConnection.Incr(time.Now().Sub(t0).Nanoseconds())
		},

		WroteRequest: func(info httptrace.WroteRequestInfo) {
			m.WriteRequest.Incr(time.Now().Sub(t0).Nanoseconds())
		},

		GotFirstResponseByte: func() {
			m.GetResponse.Incr(time.Now().Sub(t0).Nanoseconds())
		},
	})
}
