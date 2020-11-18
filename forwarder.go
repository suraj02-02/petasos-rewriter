package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/rs/zerolog/log"
)

var (
	ErrNoMatchFound = fmt.Errorf("No match found")
)

// forwarder forwads requests to real petasos instance and does
// apropriate replacements.
func forwarder(c echo.Context) error {

	log.Debug().Msg("##############################")
	log.Debug().Msg("###### Request Start #########")
	log.Debug().Msg("##############################")

	// prepare request for forwarding
	req := c.Request()

	// store scheme of original request
	originalRequestScheme := req.URL.Scheme
	if originalRequestScheme == "" {
		originalRequestScheme = req.Header.Get("X-Forwarded-Proto")
	}
	log.Debug().Msgf("originalScheme [%s]", originalRequestScheme)

	// Change protocols from ws(s) => http(s).
	// Parodus makes requests to `ws` but complains
	// when getting a redirect containing `ws`.
	switch originalRequestScheme {
	case "ws":
		log.Debug().Msgf("Replacing original scheme [%s] with [%s] in output", originalRequestScheme, "http")
		originalRequestScheme = "http"
	case "wss":
		log.Debug().Msgf("Replacing original scheme [%s] with [%s] in output", originalRequestScheme, "https")
		originalRequestScheme = "https"
	}

	dump, err := httputil.DumpRequest(req, true)
	if err != nil {
		return err
	}
	log.Debug().Msg("Dumping original request to petasos-rewriter")
	log.Debug().Msgf("%s", dump)
	log.Debug().Msg("") // br
	log.Debug().Msg("") // br

	// Prepare forwarding to petasos
	req.URL = &url.URL{
		Scheme: petasosURL.Scheme,
		Host:   petasosURL.Host,
		Path:   req.URL.Path,
	}
	req.RequestURI = ""

	// Forward to real petasos
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	dump, err = httputil.DumpRequest(req, true)
	if err != nil {
		return err
	}
	log.Debug().Msg("Dumping request to real petasos")
	log.Debug().Msgf("%s", dump)
	log.Debug().Msg("") // br
	log.Debug().Msg("") // br

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	dump, err = httputil.DumpResponse(resp, true)
	if err != nil {
		return err
	}
	log.Debug().Msg("Dumping response from real petasos")
	log.Debug().Msgf("%s", dump)
	log.Debug().Msg("") // br
	log.Debug().Msg("") // br

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// just printing the all response headers which we got from actual petasos
	for k, v := range resp.Header {
		var header string
		for _, s := range v {
			if header != "" {
				header = header + ","
			}
			header = header + s
		}
		header = strings.TrimRight(header, ",")
		c.Response().Header().Set(k, header)

		log.Debug().Msgf("k: %s, v: %s\n", k, v)
	}
	if c.Response().Status != http.StatusOK {
		return nil
	}
	// Replace location header
	location := c.Response().Header().Get("Location")
	log.Debug().Msgf("Location [%s]\n", location)

	locationUrl, err := url.Parse(location)
	if err != nil {
		return err
	}

	if *fixedScheme != "" {
		// TODO: use scheme from publicTalariaURL and make fixedScheme bool
		// locationUrl.Scheme = publicTalariaURL.Scheme
		locationUrl.Scheme = *fixedScheme
	} else {
		locationUrl.Scheme = originalRequestScheme
	}

	// Do replacement & build public talaria url
	externalTalariaName, err := replaceTalariaInternalName(
		locationUrl.Hostname(),
		*talariaInternalName,
		*talariaExternalName,
	)
	if err != nil {
		return err
	}
	publicTalariaURL := buildExternalURL(externalTalariaName, *talariaDomain)

	locationUrl.Host = publicTalariaURL
	log.Info().Msgf("redirecting from Location [%s] to Location [%s] for device name [%s] \n", location, locationUrl.String(),req.Header.Get("X-Webpa-Device-Name"))
	c.Response().Header().Set("Location", locationUrl.String())

	// Replace url in body
	var href = regexp.MustCompile(`"(.*)"`)
	body = href.ReplaceAll(body, []byte(`"`+locationUrl.String()+`"`))
	c.Response().Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))

	// Forward status code
	c.Response().Writer.WriteHeader(resp.StatusCode)

	_, err = c.Response().Writer.Write(body)
	if err != nil {
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

	// TODO: strip possible internal k8s namespace.
	// xmidt-talaria OK
	// talaria.xmidt Not OK

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
