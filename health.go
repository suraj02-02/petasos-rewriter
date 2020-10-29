package main

import (
	"net/http"
	"net/url"

	"github.com/benchkram/errz"
)

// petasosHealth return nil if petasos is reachable
func petasosHealth(u *url.URL) (err error) {
	defer errz.Recover(&err)

	// check if petasos is reachable
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequest("GET", u.String(), nil)
	errz.Fatal(err)

	// Dummy device name, petasos requires it to be set.
	req.Header.Set("X-Webpa-Device-Name", "mac:223344556677")
	_, err = client.Do(req)
	errz.Fatal(err)

	return nil
}
