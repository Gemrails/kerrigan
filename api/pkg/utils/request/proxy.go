package request

import (
	"net/http"
	"net/url"
	"time"
)

var TimeOut = 10 * time.Second

func ProxyGet(path string) (*http.Response, error) {
	rawurl := "http://127.0.0.1:12345"
	proxyUrl, err := url.Parse(rawurl)
	if err != nil {
		return nil, err
	}
	client := http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(proxyUrl)},
		Timeout:   TimeOut,
	}
	resp, err := client.Get(path)
	if err != nil || resp.StatusCode != http.StatusOK {
		return resp, err
	}
	defer resp.Body.Close()
	return resp, nil
}
