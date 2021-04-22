package main

import (
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestPetasosHealth(t *testing.T) {
	var (
		assert  = assert.New(t)
		handler = http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			assert.Equal("mac:223344556677", request.Header.Get("X-Webpa-Device-Name"))
		})
	)
	server := httptest.NewServer(handler)
	defer server.Close()
	url, _ := url.Parse(server.URL)
	err := petasosHealth(url)
	assert.Nil(err)
	url, _ = url.Parse("http://127.0.0.1:1000/")
	err = petasosHealth(url)
	assert.NotNil(err)

}
