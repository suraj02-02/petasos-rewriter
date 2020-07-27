package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

// TODO When using multiple talarias we need to do some clever replacment
// from internal to external addresses.

var port *string
var publicTalariaEndpoint *string
var petasosEndpoint *string

// if fixed scheme is set talaria
// redirects will use this scheme.
// If falls the scheme of the ortiginal request is used
var fixedScheme *string

func init() {
	// port to listen for incoming requests
	port = rootCmd.PersistentFlags().String(
		"port", "1323",
		`Port to listen on`,
	)

	// Fixed public talaria endpoint
	publicTalariaEndpoint = rootCmd.PersistentFlags().String(
		"talaria-endpoint", "http://public-talaria-domain",
		`Public talaria endpoint`,
	)

	petasosEndpoint = rootCmd.PersistentFlags().String(
		"petasos-endpoint", "",
		`Petasos endpoint, usually private.`,
	)

	fixedScheme = rootCmd.PersistentFlags().String(
		"fixed-scheme", "",
		`If set all redirects will use this scheme [http, https]`,
	)
}

var publicTalariaURL *url.URL
var petasosURL *url.URL

var rootCmd = &cobra.Command{
	Use:   "petasos-rewriter",
	Short: "",
	Long:  ``,
	Run:   Run,
}

func Run(cmd *cobra.Command, args []string) {

	// Init peasos URL
	var err error
	petasosURL, err = url.Parse(*petasosEndpoint)
	if err != nil {
		log.Error().Msg(err.Error())
		os.Exit(1)
	}

	publicTalariaURL, err = url.Parse(*publicTalariaEndpoint)
	if err != nil {
		log.Error().Msg(err.Error())
		os.Exit(1)
	}

	if !(*fixedScheme == "" || *fixedScheme == "http" || *fixedScheme == "https") {
		log.Error().Msg(fmt.Errorf("Invalid Scheme [%s]", *fixedScheme).Error())
		os.Exit(1)
	}

	fmt.Printf("Config:\n")
	fmt.Printf("  petasos-endpoint: %s\n", petasosURL.String())
	fmt.Printf("  talaria-endpoint: %s\n", publicTalariaURL.String())
	fmt.Printf("  fixed-scheme:     %s\n", *fixedScheme)
	fmt.Printf("\n\n")

	time.Sleep(10 * time.Second)
	fmt.Printf("Checking if petasos is reachable\n")

	// check if petasos is reachable
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequest("GET", petasosURL.String(), nil)
	if err != nil {
		log.Error().Msg(err.Error())
		os.Exit(1)
	}
	req.Header.Set("X-Webpa-Device-Name", "mac:223344556677")
	resp, err := client.Do(req)
	if err != nil {
		log.Error().Msg(err.Error())
		os.Exit(1)
	}
	dump, err := httputil.DumpResponse(resp, true)
	if err != nil {
		log.Error().Msg(err.Error())
		os.Exit(1)
	}
	fmt.Printf("%q", dump)

	// Echo instance
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Routes
	e.GET("/*", forwarder)

	// Start server
	e.Logger.Fatal(e.Start(":" + *port))
}

func forwarder(c echo.Context) error {
	// prepare request for forwarding
	req := c.Request()

	// store scheme of original request
	originalRequestScheme := req.URL.Scheme
	if originalRequestScheme == "" {
		originalRequestScheme = req.Header.Get("X-Forwarded-Proto")
	}
	fmt.Printf("originalScheme [%s]\n", originalRequestScheme)

	dump, err := httputil.DumpRequest(req, true)
	if err != nil {
		log.Error().Msg(err.Error())
		os.Exit(1)
	}
	fmt.Printf("Dumping original request to petasos-rewriter\n")
	fmt.Printf("%s", dump)
	fmt.Printf("\n\n")

	req.URL = &url.URL{
		Scheme: petasosURL.Scheme,
		Host:   petasosURL.Host,
		Path:   req.URL.Path,
	}
	req.RequestURI = ""

	// forward to real petasos
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	dump, err = httputil.DumpRequest(req, true)
	if err != nil {
		log.Error().Msg(err.Error())
		os.Exit(1)
	}
	fmt.Printf("Dumping request to real petasos\n")
	fmt.Printf("%s", dump)
	fmt.Printf("\n\n")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	dump, err = httputil.DumpResponse(resp, true)
	if err != nil {
		log.Error().Msg(err.Error())
		os.Exit(1)
	}
	fmt.Printf("Dumping response from real petasos\n")
	fmt.Printf("%s", dump)
	fmt.Printf("\n\n")

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

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

		fmt.Printf("k: %s, v: %s\n", k, v)
	}

	// Replace location header
	location := c.Response().Header().Get("Location")
	fmt.Printf("Location [%s]\n", location)
	locationUrl, err := url.Parse(location)
	if err != nil {
		return err
	}

	if *fixedScheme != "" {
		// TODO: use scheme from publicTalariaURL and make fixedScheme bool
		// locationUrl.Scheme = publicTalariaURL.Scheme
		locationUrl.Scheme = *fixedScheme
	}
	locationUrl.Scheme = originalRequestScheme
	locationUrl.Host = publicTalariaURL.Host
	//locationUrl.Path = publicTalariaURL.Path
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

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Error().Msg(err.Error())
		os.Exit(1)
	}

	os.Exit(0)
}
