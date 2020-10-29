package main

import (
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/avast/retry-go"
	"github.com/benchkram/errz"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

// TODO When using multiple talarias we need to do some clever replacment
// from internal to external addresses.

var port *string
var publicTalariaEndpoint *string
var petasosEndpoint *string

// When fixedScheme is set talaria
// redirects will use this scheme.
// If false, the scheme of the ortiginal request is used
var fixedScheme *string

var logFormat *string // json or plain text
var logLevel *string  // zerolog log level

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
		`If set, all redirects will use this scheme [http, https]`,
	)
	logFormat = rootCmd.PersistentFlags().String(
		"log", "json",
		`Log output format [json, text]`,
	)

	logLevel = rootCmd.PersistentFlags().String(
		"log-level", "debug",
		fmt.Sprintf("[%s,%s,%s]",
			zerolog.InfoLevel.String(),
			zerolog.DebugLevel.String(),
			zerolog.ErrorLevel.String(),
		),
	)
}

var publicTalariaURL *url.URL
var petasosURL *url.URL

var rootCmd = &cobra.Command{
	Use:   "petasos-rewriter",
	Short: "Request middleware implemented as `gateway`",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		logging()

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

		printConfig()

		// Initial health check
		log.Info().Msg("Checking if petasos is reachable")
		attempt := 1
		err = retry.Do(
			func() error {
				log.Debug().Msgf("Trying to reach petasos: [attempt: %d]", attempt)

				attempt++

				err = petasosHealth(petasosURL)
				if err != nil {
					return fmt.Errorf("unhealthy")
				}
				return nil
			},
			retry.Attempts(10),
			retry.Delay(1*time.Second),
		)
		errz.Fatal(err, "Could not reach petasos, shutting down")

		// Setup & Start Server
		e := echo.New()
		e.Use(middleware.Logger())
		e.Use(middleware.Recover())
		e.GET("/*", forwarder)
		e.Logger.Fatal(e.Start(":" + *port))
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Error().Msg(err.Error())
		os.Exit(1)
	}
	os.Exit(0)
}
