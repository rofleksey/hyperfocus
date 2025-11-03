package util

import (
	"fmt"
	"net/http"
	"net/url"
	"time"
)

func CreateProxyHttpClient(proxy string) (*http.Client, error) {
	if proxy == "" {
		return &http.Client{
			Timeout: 30 * time.Second,
		}, nil
	}

	proxyUrl, err := url.Parse(proxy)
	if err != nil {
		return nil, fmt.Errorf("proxyurl.Parse: %w", err)
	}

	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyUrl),
		},
	}, nil
}
