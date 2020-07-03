package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"

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

	// Echo instance
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Routes
	e.GET("/", forwarder)

	// Start server
	e.Logger.Fatal(e.Start(":" + *port))
}

func forwarder(c echo.Context) error {
	// prepare request for forwarding
	req := c.Request()
	req.URL = &url.URL{
		Scheme: petasosURL.Scheme,
		Host:   petasosURL.Host,
		Path:   petasosURL.Path,
	}
	req.RequestURI = ""

	// forward to real petasos
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

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
	}

	// Replace location header
	location := c.Response().Header().Get("Location")
	locationUrl, err := url.Parse(location)
	if err != nil {
		return err
	}
	locationUrl.Scheme = publicTalariaURL.Scheme
	locationUrl.Host = publicTalariaURL.Host
	locationUrl.Path = publicTalariaURL.Path
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
