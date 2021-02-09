package main

import (
	"fmt"
	sentryecho "github.com/getsentry/sentry-go/echo"
	"github.com/spf13/viper"
	"net/url"
	"os"
	"time"

	"github.com/avast/retry-go"
	"github.com/benchkram/errz"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

// When fixedScheme is set talaria  redirects will use this scheme.
// If false, the scheme of the original request is used
//var fixedScheme *string

//var logFormat *string // json or plain text
//var logLevel *string  // zerolog log level
//
//// internal domain for talaria in k8s env, replaced
//// with composition of talariaSubDomain & talariaSubDomainPrefix
//var talariaInternalName *string
//var talariaDomain *string       // domain for talaria [talaria.example.com]
//var talariaExternalName *string // external name for talaria instance
//var logDir *string // logDir Where logs will be stored
//var logFilename *string // log file where current logs will be stored.
//var sentryDsn *string // sentryDsn for capturing error's

const (
	applicationName = "petasos-rewriter"
	serverPort = "server.port"
	petasosEndpoint = "petasos.endpoint"
	talariaInternal = "talaria.internal"
	talariaExternal= "talaria.external"
	talariaDomain = "talaria.domain"
	zipkinName = "zipkin"
	jaegarName = "jaegar"
	traceProviderType = "type"
	traceProviderEndpoint = "endpoint"
	traceProviderSkipTraceExport = "skipTraceExport"
	spanIdHeader = "X-B3-SpanId"
	traceIdHeader = "X-B3-TraceId"
)

func init() {

	err := ConfigureViper(applicationName)
	if err != nil {
		errz.Fatal(err, "Could not read configuration")
	}
	//// port to listen for incoming requests
	//port = rootCmd.PersistentFlags().String(
	//	"port", "1323",
	//	`Port to listen on`,
	//)

	//petasosEndpoint = rootCmd.PersistentFlags().String(
	//	"petasos-endpoint", "http://localhost:6400",
	//	`Petasos endpoint, usually private.`,
	//)

	//logFormat = rootCmd.PersistentFlags().String(
	//	"log", "json",
	//	`Log output format [json, text, file]`,
	//)
	//logLevel = rootCmd.PersistentFlags().String(
	//	"log-level", "info",
	//	fmt.Sprintf("[%s,%s,%s]",
	//		zerolog.InfoLevel.String(),
	//		zerolog.DebugLevel.String(),
	//		zerolog.ErrorLevel.String(),
	//	),
	//)
	//
	////fixedScheme = rootCmd.PersistentFlags().String(
	////	"fixed-scheme", "",
	////	`If set, all redirects will use this scheme [http, https]`,
	////)
	//talariaInternalName = rootCmd.PersistentFlags().String(
	//	"talaria-internal", "-talaria",
	//	"Replacement candidate with talaria-external",
	//)
	//talariaDomain = rootCmd.PersistentFlags().String(
	//	"talaria-domain", "dev.rdk.yo-digital.com",
	//	"Talaria public domain to forward the request to. Example result: [replace(talaria-internal,talaria-external).talaria-domain]",
	//)
	//talariaExternalName = rootCmd.PersistentFlags().String(
	//	"talaria-external", "talaria",
	//	"Replacement for talaria-internal-name ",
	//)
	//
	//logDir = rootCmd.PersistentFlags().String(
	//	"log-dir","/tmp",
	//	"directory for storing logs",
	//	)
	//logFilename = rootCmd.PersistentFlags().String(
	//	"log-file-name" ,"petasos-rewriter.log",
	//	"Actual log file name",
	//
	//	)
	//sentryDsn = rootCmd.PersistentFlags().String(
	//	"sentry-dsn", "https://f1bbc2f55c7b4f1794abaa8f2618344e@sentry.yo-digital.com/42",
	//	"Sentry project URL",
	//	)
}

var petasosURL *url.URL
var sentryEnabled  = false

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
		fixedScheme :=  viper.GetString("server.fixedScheme")

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
