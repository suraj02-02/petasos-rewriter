package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/getsentry/sentry-go"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strings"
)

const (
	certificateProviderHeader = "X-Issuer-CN"
	expiryDateHeader          = "X-Cert-Expiry-Date"
	realIpHeader              = "X-REAL-IP"
	deviceCNHeader            = "X-DEVICE-CN"
	webpaConveyHeader         = "X-WebPA-Convey"
)

var (
	ErrNoMatchFound = fmt.Errorf("No match found")
)

// forwarder forwards requests to real petasos instance and does
// appropriate replacements.
func forwarder(c echo.Context, client *http.Client) error {
	if sentryEnabled {
		defer sentry.Recover()
	}

	req := c.Request()
	ctx := req.Context()

	if isAuthHeaderCheckEnabled && len(c.Request().Header.Get("Authorization")) == 0 {
		log.Ctx(ctx).Error().Msg("authorization header not provided")
		return c.JSON(http.StatusBadRequest, echo.NewHTTPError(http.StatusBadRequest, "authorization header not provided"))
	}

	// store scheme of original request
	originalRequestScheme := req.URL.Scheme
	if originalRequestScheme == "" {
		originalRequestScheme = req.Header.Get("X-Forwarded-Proto")
	}

	log.Ctx(ctx).Debug().Msgf("originalScheme [%s]", originalRequestScheme)

	// Change protocols from ws(s) => http(s).
	// Parodus makes requests to `ws` but complains
	// when getting a redirect containing `ws`.
	switch originalRequestScheme {
	case "ws":
		log.Ctx(ctx).Debug().Msgf("Replacing original scheme [%s] with [%s] in output", originalRequestScheme, "http")
		originalRequestScheme = "http"
		break
	case "wss":
		log.Ctx(ctx).Debug().Msgf("Replacing original scheme [%s] with [%s] in output", originalRequestScheme, "https")
		originalRequestScheme = "https"
	}

	dump, err := httputil.DumpRequest(req, true)
	if err != nil {
		panic(err)
		return err
	}
	log.Ctx(ctx).Debug().Msg("Dumping original request to petasos-rewriter")
	log.Ctx(ctx).Debug().Msgf("%s", dump)
	log.Ctx(ctx).Debug().Msg("") // br
	log.Ctx(ctx).Debug().Msg("") // br

	if remoteUpdateAddressEnabled {
		log.Ctx(ctx).Info().Msg("updating resource's IP address and certificate information")
		err := updateResourceDetails(req, client, resourceURL)
		if err != nil {
			log.Ctx(ctx).Error().Msg(err.Error())
		}
	}

	// Prepare forwarding to petasos
	req.URL = &url.URL{
		Scheme: petasosURL.Scheme,
		Host:   petasosURL.Host,
		Path:   req.URL.Path,
	}
	req.RequestURI = ""
	dump, err = httputil.DumpRequest(req, true)
	if err != nil {
		panic(err)
		return err
	}
	log.Ctx(ctx).Debug().Msg("Dumping request to real petasos")
	log.Ctx(ctx).Debug().Msgf("%s", dump)
	log.Ctx(ctx).Debug().Msg("") // br
	log.Ctx(ctx).Debug().Msg("") // br
	resp, err := client.Do(req)
	if err != nil {
		sentry.CaptureException(err)
		panic(err)
		return err
	}

	dump, err = httputil.DumpResponse(resp, true)
	if err != nil {
		sentry.CaptureException(err)
		panic(err)
		return err
	}
	log.Ctx(ctx).Debug().Msg("Dumping response from real petasos")
	log.Ctx(ctx).Debug().Msgf("%s", dump)
	log.Ctx(ctx).Debug().Msg("") // br
	log.Ctx(ctx).Debug().Msg("") // br

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		sentry.CaptureException(err)
		panic(err)
		return err
	}

	// just printing the all response headers which we got from actual petasos
	for k, v := range resp.Header {
		if k == "Traceparent" || k == "Tracestate" {
			continue
		}
		var header string
		for _, s := range v {
			if header != "" {
				header = header + ","
			}
			header = header + s
		}
		header = strings.TrimRight(header, ",")
		log.Ctx(ctx).Debug().Msgf("k: %s, v: %s\n", k, v)
		c.Response().Header().Set(k, header)
	}

	if resp.StatusCode != http.StatusTemporaryRedirect {
		// Forward status code
		c.Response().Writer.WriteHeader(resp.StatusCode)
		c.Response().Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
		c.Response().Writer.Write(body)
		return nil
	}
	// Replace location header
	location := c.Response().Header().Get("Location")
	log.Ctx(ctx).Debug().Msgf("Location [%s]\n", location)

	locationUrl, err := url.Parse(location)
	if err != nil {
		sentry.CaptureException(err)
		panic(err)
		return err
	}
	fixedScheme := viper.GetString("server.fixedScheme")

	if fixedScheme != "" {
		// TODO: use scheme from publicTalariaURL and make fixedScheme bool
		// locationUrl.Scheme = publicTalariaURL.Scheme
		locationUrl.Scheme = fixedScheme
	} else {
		locationUrl.Scheme = originalRequestScheme
	}

	// Do replacement & build public talaria url
	externalTalariaName, err := replaceTalariaInternalName(
		locationUrl.Hostname(),
		viper.GetString(talariaInternal),
		viper.GetString(talariaExternal),
	)
	if err != nil {
		sentry.CaptureException(err)
		return err
	}
	publicTalariaURL := buildExternalURL(externalTalariaName, viper.GetString(talariaDomain))

	locationUrl.Host = publicTalariaURL
	log.Ctx(ctx).Info().Msgf("redirecting from Location [%s] to Location [%s] for device name [%s] \n", location, locationUrl.String(), req.Header.Get("X-Webpa-Device-Name"))
	c.Response().Header().Set("Location", locationUrl.String())

	// Replace url in body
	var href = regexp.MustCompile(`"(.*)"`)
	body = href.ReplaceAll(body, []byte(`"`+locationUrl.String()+`"`))
	c.Response().Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))

	// Forward status code
	c.Response().Writer.WriteHeader(resp.StatusCode)

	_, err = c.Response().Writer.Write(body)
	if err != nil {
		sentry.CaptureException(err)
		panic(err)
		return err
	}

	return nil
}

// replaceTalariaInternalName replaces internal talaria name.
// Returns a ErrNoMatchFound when replacement is impossible.
func replaceTalariaInternalName(host, old, new string) (string, error) {
	index := strings.Index(host, old)
	if index == -1 {
		return "", ErrNoMatchFound
	}
	talariaExternal := strings.Replace(host, old, new, -1)
	return talariaExternal, nil
}

// buildExternalURL by concatenation new talaria name + given domain
func buildExternalURL(newTalariaName, domain string) string {
	var builder strings.Builder
	builder.WriteString(newTalariaName)
	builder.WriteString(".")
	builder.WriteString(domain)
	return builder.String()
}

/*
 * Extracts IP address, certificate provider, and expiry details from HTTP request headers.
 * Calls the RI API with this information.
 */
func base64Decode(encodedStr string) ([]byte, error) {
	decodedData, err := base64.StdEncoding.DecodeString(encodedStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 string: %v", err)
	}
	return decodedData, nil
}

func populateWebPaConveyHeaderDataIfPresent(webPAConveyHeader string, updatedResourceRequestBody *UpdateResourceRequest) error {
	if len(webPAConveyHeader) > 0 {
		decodedData, err := base64Decode(webPAConveyHeader)
		if err != nil {
			return err
		}

		var conveyHeaderData WebPAConveyHeaderData
		if err := json.Unmarshal(decodedData, &conveyHeaderData); err != nil {
			return fmt.Errorf("failed to unmarshal decoded X-WebPA-Convey data: %v", err)
		}

		updatedResourceRequestBody.LastRebootReason = conveyHeaderData.HwLastRebootReason
		updatedResourceRequestBody.WanInterfaceUsed = conveyHeaderData.WebpaInterfaceUsed
		updatedResourceRequestBody.LastReconnectReason = conveyHeaderData.WebpaLastReconnectReason
		updatedResourceRequestBody.ManagementProtocol = conveyHeaderData.WebpaProtocol
		updatedResourceRequestBody.LastBootTime = conveyHeaderData.BootTime
		updatedResourceRequestBody.FirmwareVersion = conveyHeaderData.FwName
	}
	return nil
}

func updateResourceDetails(req *http.Request, client *http.Client, resourceURL *url.URL) error {
	certificateProviderRaw := req.Header.Get(certificateProviderHeader)
	certificateProviderType := "DTSECURITY"
	if strings.Contains(certificateProviderRaw, "C2") {
		certificateProviderType = "IRDETO"
	}

	requestBody := UpdateResourceRequest{
		IpAddress:               req.Header.Get(realIpHeader),
		CertificateProviderType: certificateProviderType,
		CertificateExpiryDate:   req.Header.Get(expiryDateHeader),
	}

	webPAConveyHeader := req.Header.Get(webpaConveyHeader)
	err := populateWebPaConveyHeaderDataIfPresent(webPAConveyHeader, &requestBody)
	if err != nil {
		return err
	}

	log.Ctx(req.Context()).Info().Msgf("Certificate Provider type: [%s], Certificate expiry date: [%s], HW Last Reboot Reason: [%s], Webpa Interface Used: [%s], Webpa Last Reconnect Reason: [%s], Webpa Protocol: [%s], Last Boot Time: [%d], Firmware Version: [%s]",
		requestBody.CertificateProviderType, requestBody.CertificateExpiryDate,
		requestBody.LastRebootReason, requestBody.WanInterfaceUsed,
		requestBody.LastReconnectReason, requestBody.ManagementProtocol,
		requestBody.LastBootTime, requestBody.FirmwareVersion)

	cpeIdentifier := strings.ToLower(req.Header.Get(deviceCNHeader))
	jsonBytes, err := json.Marshal(requestBody)
	if err != nil {
		return err
	}

	//resourceURL = abc.com/v1/resource/macAddress
	finalUrl := resourceURL.String() + "/" + cpeIdentifier
	request, err := http.NewRequest(http.MethodPut, finalUrl, bytes.NewReader(jsonBytes))
	if err != nil {
		return err
	}
	request.Header.Add("Content-Type", "application/json")
	request.Header.Set("ENVIRONMENT", req.Header.Get("ENVIRONMENT"))
	request.Header.Set("X-TENANT-ID", req.Header.Get("X-TENANT-ID"))

	resp, err := client.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status code received while updating resource's IP address via Petasos rewriter: %d", resp.StatusCode)
	}

	return nil
}
