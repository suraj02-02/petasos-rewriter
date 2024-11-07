package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestReplaceTalariaInternalName(t *testing.T) {

	testData := []struct {
		host     string
		old      string
		new      string
		expected string
		err      error
	}{
		{"xmidt-talaria-1", "xmidt-talaria-", "talaria", "talaria1", nil},
		{"xmidt-talaria-2", "xmidt-talaria-", "talaria", "talaria2", nil},
		{
			host:     "xmidt-talaria3",
			old:      "xmidt-talaria",
			new:      "talaria",
			expected: "talaria3",
		},
		{
			host:     "xmidt-talaria4",
			old:      "xmidt-talaria",
			new:      "talaria",
			expected: "talaria4",
		},
		{"xmidt-talaria4", "xmidt-talaria-", "talaria", "talaria4", ErrNoMatchFound},
	}

	for i, record := range testData {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			var (
				assert      = assert.New(t)
				actual, err = replaceTalariaInternalName(record.host, record.old, record.new)
			)
			if err != nil {
				assert.Equal(record.err, err)
			} else {
				assert.Equal(record.expected, actual)
			}
		})
	}

}

func TestBuildExternalURL(t *testing.T) {
	testData := []struct {
		arg1     string
		arg2     string
		expected string
	}{
		{"", "", "."},
		{"talaria", "Test.com", "talaria.Test.com"},
		{"talaria2", "dev.rdk.yo-digital.com", "talaria2.dev.rdk.yo-digital.com"},
		{"talaria3", "xyz.com", "talaria3.xyz.com"},
	}
	for i, record := range testData {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			var (
				assert = assert.New(t)
				actual = buildExternalURL(record.arg1, record.arg2)
			)
			assert.Equal(record.expected, actual)

		})
	}

}

func TestForwarder(t *testing.T) {
	testData := []struct {
		deviceName string
	}{
		{"mac:B827EBB25F81"},
		{"mac:B827EBB25F82"},
		{"mac:B827EBB25F83"},
		{"mac:B827EBB25F84"},
		{"mac:B827EBB25F85"},
		{"mac:B827EBB25F86"},
		{"mac:B827EBB25F87"},
	}
	e := echo.New()

	for i, record := range testData {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			var (
				assert  = assert.New(t)
				handler = http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
					assert.Equal(record.deviceName, request.Header.Get("X-Webpa-Device-Name"))
					response.Header().Set("Content-Type", "text/html; charset=utf-8")
					response.Header().Set("Location", "http://xmidt-talaria:6200/api/v2/device")
					response.Header().Set("Date", time.Now().String())
					response.Header().Set("X-Petasos-Build", "Test")
					response.Header().Set("X-Petasos-Flavor", "Test")
					response.Header().Set("X-Petasos-Region", "Test")
					response.Header().Set("X-Petasos-Server", "Test")
					response.Header().Set("X-Webpa-Device-Name", record.deviceName)
					body := "<a href=\"http://xmidt-talaria:6200/api/v2/device\">Temporary Redirect</a>.\n"
					response.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
					response.Write([]byte(body))
				})
			)

			server := httptest.NewServer(handler)
			defer server.Close()
			petasosURL, _ = url.Parse(server.URL)
			r := httptest.NewRequest("", "/v2/api/device", nil)
			r.Header.Set("X-Webpa-Device-Name", record.deviceName)
			r.Header.Set("X-Forwarded-Proto", "ws")
			client := &http.Client{
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					return http.ErrUseLastResponse
				},
			}
			w := httptest.NewRecorder()
			c := e.NewContext(r, w)
			err := forwarder(c, client)
			assert.Nil(err)

		})
	}
}
func TestUpdateResourceDetails(t *testing.T) {
	testsData := []struct {
		description                       string
		realIP                            string
		certificateProvider               string
		certificateExpiryDate             string
		deviceCN                          string
		webpaConveyHeader                 string
		expectedUpdateResourceRequestBody string
		expectedStatus                    int
	}{
		{
			description:                       "Valid input with all fields populated",
			realIP:                            "127.0.0.1",
			certificateProvider:               "DTSECURITY",
			certificateExpiryDate:             "Sep 19 23:59:59 2031 GMT",
			deviceCN:                          "TestCPE",
			webpaConveyHeader:                 "eyJody1tb2RlbCI6IlwiRkdBMjIzM1wiIiwiaHctc2VyaWFsLW51bWJlciI6IjIyMzNBRENNTCIsImh3LW1hbnVmYWN0dXJlciI6IlwiVGVjaG5pY29sb3JcIiIsImZ3LW5hbWUiOiIwMDUuMDMzLjAwMSIsImJvb3QtdGltZSI6MTcyNTAwMDYwOCwid2VicGEtcHJvdG9jb2wiOiJQQVJPRFVTLTIuMC02MWIxYTdhIiwid2VicGEtaW50ZXJmYWNlLXVzZWQiOiJlcm91dGVyMCIsImh3LWxhc3QtcmVib290LXJlYXNvbiI6InVua25vd24iLCJ3ZWJwYS1sYXN0LXJlY29ubmVjdC1yZWFzb24iOiJTU0xfU29ja2V0X0Nsb3NlIn0=",
			expectedUpdateResourceRequestBody: `{"ipAddress":"127.0.0.1","certificateProviderType":"DTSECURITY","certificateExpiryDate":"Sep 19 23:59:59 2031 GMT","lastRebootReason":"unknown","wanInterfaceUsed":"erouter0","lastReconnectReason":"SSL_Socket_Close","managementProtocol":"PARODUS-2.0-61b1a7a","lastBootTime":"2024-08-30T12:20:08+05:30","firmwareVersion":"005.033.001"}`,
			expectedStatus:                    http.StatusOK,
		},
		{
			description:                       "Input with empty IP address",
			realIP:                            "",
			certificateProvider:               "C2 CertProvider",
			certificateExpiryDate:             "Dec 31 23:59:59 2025 GMT",
			deviceCN:                          "TestCPE",
			webpaConveyHeader:                 "eyJody1tb2RlbCI6IlwiQ2hhcmFjdGVyMS5NRSIsImh3LW1hbnVmYWN0dXJlciI6IlwiU3RhcHRvbXNcIiIsImJvb3QtdGltZSI6MTY2NzkwMDAwMCwid2VicGEtcHJvdG9jb2wiOiJQQVJPRFVTLTEuMC0xYjJhZDU3MCIsIndlYnBhLWFuZGVyeWRlLXNlY3RvciI6InRlc3RfcHJvdG90YWN0dXJlIiwiZ3VhZ2UtdHlwZSI6InVua25vd24ifQ==",
			expectedUpdateResourceRequestBody: `{"ipAddress":"","certificateProviderType":"IRDETO","certificateExpiryDate":"Dec 31 23:59:59 2025 GMT","managementProtocol":"PARODUS-1.0-1b2ad570","lastBootTime":"2022-11-08T15:03:20+05:30"}`,
			expectedStatus:                    http.StatusOK,
		},
		{
			description:                       "Input with specific certificate provider",
			realIP:                            "127.0.0.1",
			certificateProvider:               "CertProvider",
			certificateExpiryDate:             "Dec 31 23:59:59 2025 GMT",
			deviceCN:                          "TestCPE",
			webpaConveyHeader:                 "eyJody1tb2RlbCI6IlwiQ2hhcmFjdGVyMS5MRSIsImh3LW1hbnVmYWN0dXJlciI6IlwiVGVjaG5pY29sb3JcIiIsImZ3LW5hbWUiOiIwMDUuMDMzLjAwMSIsImJvb3QtdGltZSI6MTY2NzkwMDAwMCwid2VicGEtcHJvdG9jb2wiOiJQQVJPRFVTLTEuMC0xYjJhZDU3MCIsIndlYnBhLWFuZGVyeWRlLXNlY3RvciI6InRlc3RfcHJvdG90YWN0dXVyZSIsImh3LWxhc3QtcmVib290LXJlYXNvbiI6InVua25vd24ifQ==",
			expectedUpdateResourceRequestBody: `{"ipAddress":"127.0.0.1","certificateProviderType":"DTSECURITY","certificateExpiryDate":"Dec 31 23:59:59 2025 GMT","lastRebootReason":"unknown","managementProtocol":"PARODUS-1.0-1b2ad570","lastBootTime":"2022-11-08T15:03:20+05:30","firmwareVersion":"005.033.001"}`,
			expectedStatus:                    http.StatusOK,
		},
		{
			description:                       "Input with empty certificate expiry date",
			realIP:                            "127.0.0.1",
			certificateProvider:               "DTSECURITY",
			certificateExpiryDate:             "",
			deviceCN:                          "TestCPE",
			webpaConveyHeader:                 "eyJody1tb2RlbCI6IlwiRkdBMjIzM1wiIiwiaHctc2VyaWFsLW51bWJlciI6IjIyMzNBRENNTCIsImh3LW1hbnVmYWN0dXJlciI6IlwiVGVjaG5pY29sb3JcIiIsImZ3LW5hbWUiOiIwMDUuMDMzLjAwMSIsImJvb3QtdGltZSI6MTcyNTAwMDYwOCwid2VicGEtcHJvdG9jb2wiOiJQQVJPRFVTLTIuMC02MWIxYTdhIiwid2VicGEtaW50ZXJmYWNlLXVzZWQiOiJlcm91dGVyMCIsImh3LWxhc3QtcmVib290LXJlYXNvbiI6InVua25vd24ifQ==",
			expectedUpdateResourceRequestBody: `{"ipAddress":"127.0.0.1","certificateProviderType":"DTSECURITY","certificateExpiryDate":"","lastRebootReason":"unknown","wanInterfaceUsed":"erouter0","managementProtocol":"PARODUS-2.0-61b1a7a","lastBootTime":"2024-08-30T12:20:08+05:30","firmwareVersion":"005.033.001"}`,
			expectedStatus:                    http.StatusOK,
		},
		{
			description:                       "Bad request error due to invalid request",
			realIP:                            "127.0.0.1",
			certificateProvider:               "DTSECURITY",
			certificateExpiryDate:             "Sep 19 23:59:59 2031 GMT",
			deviceCN:                          "TestCPE",
			webpaConveyHeader:                 "eyJjb250ZXh0IjoiY2VydGlmaWNhdGVFeHBpcnlEYXRlIjoiU2VwIDE5IDIzOjU5OjU5IDIwMzE4IEdNVCIsImNlcnRpZmljYXRlUHJvdmlkZXIiOiJEVFNFQ1VSSVRZIiwiaHctbWFudWZhY3R1cmVyIjoiUEFSQU1PVVQtMi4wLTYxYjFhN2EiLCJmb3JtYXR0aW9uIjoiMDA1LjAzMy4wMDEiLCJib290LXRpbWUiOjE3MjUwMDA2MDgsIndlYnBhLXByb3RvY29sIjoiUEFSQU1PVVQtMi4wLTYxYjFhN2EiLCJ3ZWJwYS1sYXN0LXJlY29ubmVjdC1yZWFzb24iOiJTU0xfU29ja2V0X0Nsb3NlIn0=",
			expectedUpdateResourceRequestBody: `{"ipAddress":"127.0.0.1","certificateProviderType":"DTSECURITY","certificateExpiryDate":"Sep 19 23:59:59 2031 GMT"}`,
			expectedStatus:                    http.StatusBadRequest,
		},
		{
			description:                       "Invalid X-WebPA-Convey header",
			realIP:                            "127.0.0.1",
			certificateProvider:               "DTSECURITY",
			certificateExpiryDate:             "Sep 19 23:59:59 2031 GMT",
			deviceCN:                          "TestCPE",
			webpaConveyHeader:                 "abcd1234",
			expectedUpdateResourceRequestBody: "",
			expectedStatus:                    http.StatusBadRequest,
		},
		{
			description:                       "No X-WebPA-Convey header present",
			realIP:                            "192.168.1.1",
			certificateProvider:               "DTSECURITY",
			certificateExpiryDate:             "Dec 31 23:59:59 2025 GMT",
			deviceCN:                          "TestCPE",
			webpaConveyHeader:                 "",
			expectedUpdateResourceRequestBody: `{"ipAddress":"192.168.1.1","certificateProviderType":"DTSECURITY","certificateExpiryDate":"Dec 31 23:59:59 2025 GMT"}`,
			expectedStatus:                    http.StatusOK,
		},
	}

	for i, tt := range testsData {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			assert := assert.New(t)

			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(http.MethodPut, r.Method)
				requestBody, err := io.ReadAll(r.Body)
				assert.NoError(err)
				assert.JSONEq(tt.expectedUpdateResourceRequestBody, string(requestBody))
				w.WriteHeader(tt.expectedStatus)
			}))
			defer mockServer.Close()

			testReq, err := http.NewRequest(http.MethodPut, "/", nil)
			assert.NoError(err)
			testReq.Header.Set(realIpHeader, tt.realIP)
			testReq.Header.Set(certificateProviderHeader, tt.certificateProvider)
			testReq.Header.Set(expiryDateHeader, tt.certificateExpiryDate)
			testReq.Header.Set(deviceCNHeader, tt.deviceCN)
			testReq.Header.Set(webpaConveyHeader, tt.webpaConveyHeader)
			testReq.Header.Set("ENVIRONMENT", "test")
			testReq.Header.Set("X-TENANT-ID", "12345")

			client := &http.Client{
				Transport: &http.Transport{
					Proxy: func(req *http.Request) (*url.URL, error) {
						return url.Parse(mockServer.URL)
					},
				},
			}

			resourceURL, err := url.Parse(mockServer.URL + "/v1/resource/macAddress")
			assert.NoError(err)

			err = updateResourceDetails(testReq, client, resourceURL)

			if tt.expectedStatus != http.StatusOK {
				assert.Error(err)
			} else {
				assert.NoError(err)
			}
		})
	}
}
