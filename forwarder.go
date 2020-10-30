package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/rs/zerolog/log"
)

var(

	noMatchFound = errors.New("no match found")
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
		return  err
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

	publicTalariaURL, err := parseHost(locationUrl.Host)
	if err != nil {
		return  err
	}
	locationUrl.Host = publicTalariaURL
	log.Info().Msgf("redirecting from Location [%s] to Location [%s] \n", location,locationUrl.String())
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


func parseHost(host string) (string, error) {
	h, _, err := net.SplitHostPort(host)
	if err != nil {
		return "", err
	}
	index := strings.Index(h,*talariaInternalHostName)
	if index == -1 {
		return "", noMatchFound
	}
	newHost := strings.Replace(h,*talariaInternalHostName,*talariaSubDomainPrefix,-1)
	var builder  strings.Builder
	builder.WriteString(newHost)
	builder.WriteString(".")
	builder.WriteString(*talariaSubDomain)
	return  builder.String(),nil
}
