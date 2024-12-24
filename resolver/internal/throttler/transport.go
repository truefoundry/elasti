package throttler

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/http2"
	"k8s.io/apimachinery/pkg/util/wait"
)

type RoundTripperFunc func(*http.Request) (*http.Response, error)

func (rt RoundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return rt(r)
}

// Errors
var ErrTimeoutDialing = errors.New("timed out dialing")

// Config
const sleep = 30 * time.Millisecond

var DialWithBackOff = NewBackoffDialer(backOffTemplate)
var backOffTemplate = wait.Backoff{
	Duration: 50 * time.Millisecond,
	Factor:   1.4,
	Jitter:   0.1, // At most 10% jitter.
	Steps:    15,
}

func NewProxyAutoTransport(maxIdleProxyConns, maxIdleProxyConnsPerHost int) http.RoundTripper {
	v1 := newHTTPTransport(false /*disable keep-alives*/, true /*disable auto-compression*/, maxIdleProxyConns, maxIdleProxyConnsPerHost)
	v2 := newH2CTransport(true)
	return RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
		t := v1
		if r.ProtoMajor == 2 {
			t = v2
		}
		return t.RoundTrip(r)
	})
}

func newHTTPTransport(disableKeepAlives, disableCompression bool, maxIdle, maxIdlePerHost int) http.RoundTripper {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = DialWithBackOff
	transport.DisableKeepAlives = disableKeepAlives
	transport.MaxIdleConns = maxIdle
	transport.MaxIdleConnsPerHost = maxIdlePerHost
	transport.ForceAttemptHTTP2 = false
	transport.DisableCompression = disableCompression
	return transport
}

func newH2CTransport(disableCompression bool) http.RoundTripper {
	return &http2.Transport{
		AllowHTTP:          true,
		DisableCompression: disableCompression,
		DialTLS: func(netw, addr string, _ *tls.Config) (net.Conn, error) {
			return DialWithBackOff(context.Background(),
				netw, addr)
		},
	}
}

func NewBackoffDialer(backoffConfig wait.Backoff) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		return dialBackOffHelper(ctx, network, address, backoffConfig, nil)
	}
}

func dialBackOffHelper(ctx context.Context, network, address string, bo wait.Backoff, tlsConf *tls.Config) (net.Conn, error) {
	dialer := &net.Dialer{
		Timeout:   bo.Duration, // Initial duration.
		KeepAlive: 5 * time.Second,
		DualStack: true,
	}
	start := time.Now()
	for {
		var (
			c   net.Conn
			err error
		)
		if tlsConf == nil {
			c, err = dialer.DialContext(ctx, network, address)
		} else {
			c, err = tls.DialWithDialer(dialer, network, address, tlsConf)
		}
		if err != nil {
			var errNet net.Error
			if errors.As(err, &errNet) && errNet.Timeout() {
				if bo.Steps < 1 {
					break
				}
				dialer.Timeout = bo.Step()
				time.Sleep(wait.Jitter(sleep, 1.0)) // Sleep with jitter.
				continue
			}
			return nil, fmt.Errorf("dialBackOffHelper: %w", err)
		}
		return c, nil
	}
	elapsed := time.Since(start)
	return nil, fmt.Errorf("%w %s after %.2fs", ErrTimeoutDialing, address, elapsed.Seconds())
}
