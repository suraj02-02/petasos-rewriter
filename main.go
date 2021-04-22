package main

import (
	"fmt"
	"github.com/avast/retry-go"
	"github.com/benchkram/errz"
	"github.com/getsentry/sentry-go"
	sentryecho "github.com/getsentry/sentry-go/echo"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"net/url"
	"os"
	"time"
)

const (
	applicationName              = "petasos-rewriter"
	serverPort                   = "server.port"
	petasosEndpoint              = "petasos.endpoint"
	talariaInternal              = "talaria.internal"
	talariaExternal              = "talaria.external"
	talariaDomain                = "talaria.domain"
	zipkinName                   = "zipkin"
	jaegarName                   = "jaegar"
	traceProviderType            = "type"
	traceProviderEndpoint        = "endpoint"
	traceProviderSkipTraceExport = "skipTraceExport"
	spanIdHeader                 = "X-B3-SpanId"
	traceIdHeader                = "X-B3-TraceId"
)

func init() {

	err := ConfigureViper(applicationName)
	if err != nil {
		errz.Fatal(err, "Could not read configuration")
	}

}

var petasosURL *url.URL
var sentryEnabled = false

var rootCmd = &cobra.Command{
	Use:   "petasos-rewriter",
	Short: "Request middleware implemented as `gateway`",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		printConfig()
		logging(viper.Sub("log"))

		var err error
		petasosURL, err = url.Parse(viper.GetString(petasosEndpoint))
		if err != nil {
			log.Error().Msg(err.Error())
			os.Exit(1)
		}
		fixedScheme := viper.GetString("server.fixedScheme")

		if !(fixedScheme == "" || fixedScheme == "http" || fixedScheme == "https") {
			log.Error().Msg(fmt.Errorf("Invalid Scheme [%s]", fixedScheme).Error())
			os.Exit(1)
		}

		ConfigureSentry(viper.Sub("sentry"))
		configureTracerProvider(viper.Sub("traceProvider"), applicationName)

		// Initial health check
		log.Info().Msg("Checking if petasos is reachable")

		attempt := 1
		err = retry.Do(
			func() error {
				log.Debug().Msgf("Trying to reach petasos: [attempt: %d]", attempt)

				attempt++

				err = petasosHealth(petasosURL)
				if err != nil {
					sentry.CaptureException(err)
					sentry.Flush(2 * time.Second)
					return fmt.Errorf("unhealthy")
				}
				return nil
			},
			retry.Attempts(10),
			retry.Delay(1*time.Second),
		)
		sentry.WithScope(func(scope *sentry.Scope) {
			scope.SetLevel(sentry.LevelFatal)
			sentry.CaptureMessage("Could not reach petasos, shutting down")
		})
		errz.Fatal(err, "Could not reach petasos, shutting down")

		// Setup & Start Server
		e := echo.New()
		e.Use(middleware.Logger())
		e.Use(middleware.Recover())
		if sentryEnabled {
			e.Use(sentryecho.New(sentryecho.Options{
				Repanic: true,
			}))

		}
		e.GET("/*", forwarder)
		e.Logger.Fatal(e.Start(":" + viper.GetString(serverPort)))
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Error().Msg(err.Error())
		os.Exit(1)
	}
	os.Exit(0)
}
