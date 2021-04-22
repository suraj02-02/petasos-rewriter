package main

import (
	"fmt"
	"github.com/benchkram/errz"
	"github.com/getsentry/sentry-go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout"
	"go.opentelemetry.io/otel/exporters/trace/jaeger"
	"go.opentelemetry.io/otel/exporters/trace/zipkin"
	"go.opentelemetry.io/otel/label"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"gopkg.in/natefinch/lumberjack.v2"
	"io"
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

/***
 1. will be responsible for creating the traceprovider and setting it back to  opentelemetry
		viper will be having all the fields like  type,endpoint and skipTraceExport
 2. supported traceProviders are zipkin,jaegar and stdout
 3. set skipTraceExport = true if you don't want to print the span and tracer information in stdout
*/
func configureTracerProvider(v *viper.Viper, applicationName string) {
	var traceProviderName = v.GetString(traceProviderType)

	switch traceProviderName {

	case zipkinName:
		err := zipkin.InstallNewPipeline(
			v.GetString(traceProviderEndpoint),
			applicationName,
			zipkin.WithSDK(&sdktrace.Config{DefaultSampler: sdktrace.AlwaysSample()}),
		)
		if err != nil {
			log.Debug().Msg("failed to create zipkin pipeline")
		}
		break
	case jaegarName:
		flush, err := jaeger.InstallNewPipeline(
			jaeger.WithCollectorEndpoint(v.GetString(traceProviderEndpoint)),
			jaeger.WithProcess(jaeger.Process{
				ServiceName: applicationName,
				Tags: []label.KeyValue{
					label.String("exporter", jaegarName),
				},
			}),
			jaeger.WithSDK(&sdktrace.Config{DefaultSampler: sdktrace.AlwaysSample()}),
		)
		if err != nil {
			log.Debug().Msg("failed to create jaegar pipeline")
		}
		defer flush()
		break
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
			log.Debug().Msg("failed to create stdout exporter")
			return
		}
		traceProvider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(otExporter))
		otel.SetTracerProvider(traceProvider)
	}
}
