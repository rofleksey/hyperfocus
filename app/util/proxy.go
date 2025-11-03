package util

import (
	"fmt"
	"net/http"
	"net/url"
	"sync"
)

type RotatingProxyTransport struct {
	proxies []*url.URL
	current int
	mu      sync.Mutex
	base    http.RoundTripper
}

func NewRotatingProxyTransport(urls []string) (http.RoundTripper, error) {
	if len(urls) == 0 {
		return &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
		}, nil
	}

	proxies := make([]*url.URL, 0, len(urls))
	for _, u := range urls {
		proxy, err := url.Parse(u)
		if err != nil {
			return nil, fmt.Errorf("error parsing proxy URL %s: %w", u, err)
		}

		proxies = append(proxies, proxy)
	}

	return &RotatingProxyTransport{
		proxies: proxies,
		base: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
		},
	}, nil
}

func (r *RotatingProxyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r.mu.Lock()
	proxy := r.proxies[r.current]
	r.current = (r.current + 1) % len(r.proxies)
	r.mu.Unlock()

	// Clone the base transport and set proxy
	transport := r.base
	if transport == nil {
		transport = http.DefaultTransport.(*http.Transport).Clone()
	}

	transport.(*http.Transport).Proxy = http.ProxyURL(proxy)
	return transport.RoundTrip(req)
}
