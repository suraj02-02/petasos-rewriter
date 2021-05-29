package main

import (
	"fmt"
	"github.com/benchkram/errz"
	"github.com/getsentry/sentry-go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout"
	"go.opentelemetry.io/otel/exporters/trace/jaeger"
	"go.opentelemetry.io/otel/exporters/trace/zipkin"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/semconv"
	"go.opentelemetry.io/otel/trace"
	"gopkg.in/natefinch/lumberjack.v2"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
)

func logging(v *viper.Viper) {
	switch v.GetString("level") {
	case zerolog.DebugLevel.String():
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case zerolog.InfoLevel.String():
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case zerolog.ErrorLevel.String():
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}
	logType := v.GetString("type")
	json := v.GetBool("json")
	if logType == "file" {
		log.Logger = log.Output(fileAppender(v)).With().Caller().Logger()
	} else if !json {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()
	}

}

// printConfig read from cli to stdout
func printConfig() {
	fmt.Printf("Config:\n")
	for _, s := range viper.AllKeys() {
		fmt.Printf("  %s :                %s\n", s, viper.Get(s))
	}
	fmt.Printf("\n\n")
}

func fileAppender(v *viper.Viper) io.Writer {
	dir := v.GetString("dir")
	if err := os.MkdirAll(dir, 0744); err != nil {
		log.Error().Err(err).Str("path", dir).Msg("can't create log directory")
		return nil
	}
	return &lumberjack.Logger{
		Filename:   path.Join(dir, v.GetString("fileName")),
		MaxSize:    v.GetInt("maxSize"),
		MaxAge:     v.GetInt("maxAge"),
		MaxBackups: v.GetInt("maxBackups"),
	}

}

func ConfigureSentry(v *viper.Viper) {
	sentryDsn := v.GetString("dsn")
	if sentryDsn == "NA" {
		return
	}
	err := sentry.Init(sentry.ClientOptions{
		Dsn:              sentryDsn,
		Environment:      v.GetString("environment"),
		Debug:            v.GetBool("debug"),
		AttachStacktrace: true,
	})
	sentry.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetTags(map[string]string{"app_name": applicationName})
		scope.SetLevel(sentry.LevelError)
	})

	if err != nil {
		errz.Fatal(err, "Could not configure sentry, shutting down")
	}
	sentryEnabled = true
	defer sentry.Flush(2 * time.Second)

}

func ConfigureViper(applicationName string) error {
	viper.AddConfigPath(fmt.Sprintf("/etc/%s", applicationName))
	viper.AddConfigPath(fmt.Sprintf("$HOME/.%s", applicationName))
	viper.AddConfigPath(".")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.SetEnvPrefix(applicationName)
	viper.AutomaticEnv()
	viper.SetConfigName(applicationName)
	err := viper.ReadInConfig()
	return err
}

// ConfigureTracerProvider creates the TracerProvider based on the configuration
// provided. It has built-in support for jaeger, zipkin, stdout and noop providers.
// A different provider can be used if a constructor for it is provided in the
// config.
// If a provider name is not provided, a stdout tracerProvider will be returned.
func configureTracerProvider(v *viper.Viper, applicationName string) (trace.TracerProvider, error) {
	var traceProviderName = v.GetString(traceProviderType)
	switch traceProviderName {

	case zipkinName:
		traceProvider, err := zipkin.NewExportPipeline(v.GetString(traceProviderEndpoint),
			zipkin.WithSDKOptions(
				sdktrace.WithSampler(sdktrace.AlwaysSample()),
				sdktrace.WithResource(
					resource.NewWithAttributes(semconv.ServiceNameKey.String(applicationName)),
				),
			),
		)
		return traceProvider, err
	case jaegarName:
		traceProvider, _, err := jaeger.NewExportPipeline(
			jaeger.WithCollectorEndpoint(v.GetString(traceProviderEndpoint)),
			jaeger.WithSDKOptions(
				sdktrace.WithSampler(sdktrace.AlwaysSample()),
				sdktrace.WithResource(
					resource.NewWithAttributes(
						semconv.ServiceNameKey.String(applicationName),
						attribute.String("exporter", traceProviderName),
					),
				),
			),
		)
		if err != nil {
			return nil, err
		}
		return traceProvider, nil
	case noopName:
		return trace.NewNoopTracerProvider(), nil
	default:
		var skipTraceExport = v.GetBool(traceProviderSkipTraceExport)
		var option stdout.Option
		if skipTraceExport {
			option = stdout.WithoutTraceExport()
		} else {
			option = stdout.WithPrettyPrint()
		}
		otExporter, err := stdout.NewExporter(option)
		if err != nil {
			return nil, err
		}
		traceProvider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(otExporter))
		return traceProvider, nil
	}
}

func configureClient(propagators propagation.TextMapPropagator, provider trace.TracerProvider) *http.Client {
	var transport http.RoundTripper = &http.Transport{}
	transport = otelhttp.NewTransport(transport,
		otelhttp.WithPropagators(propagators),
		otelhttp.WithTracerProvider(provider),
	)

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: transport,
	}
	return client

}
