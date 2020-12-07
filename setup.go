package main

import (
	"fmt"
	"io"
	"os"
	"path"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/natefinch/lumberjack.v2"
)

func logging() {
	switch *logLevel {
	case zerolog.DebugLevel.String():
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case zerolog.InfoLevel.String():
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case zerolog.ErrorLevel.String():
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}
	if *logFormat == "text" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()
	}else if  *logFormat == "file" {
		log.Logger = log.Output(fileAppender(*logDir,*logFilename)).With().Caller().Logger()
	}

}

// printConfig read from cli to stdout
func printConfig() {
	fmt.Printf("Config:\n")
	fmt.Printf("  petasos-endpoint:   %s\n", petasosURL.String())
	fmt.Printf("  fixed-scheme:       %s\n", *fixedScheme)
	fmt.Printf("  log:                %s\n", *logFormat)
	fmt.Printf("  log-level:          %s\n", *logLevel)
	fmt.Printf("  talaria-domain: %s\n", *talariaDomain)
	fmt.Printf("  talaria-internal-name: %s\n", *talariaInternalName)
	fmt.Printf("  talaria-external-name: %s\n", *talariaExternalName)
	fmt.Printf("\n\n")
}

func fileAppender(dir,filename string) io.Writer {
	if err := os.MkdirAll(dir, 0744); err != nil {
		log.Error().Err(err).Str("path", dir).Msg("can't create log directory")
		return nil
	}
	return &lumberjack.Logger{
		Filename:   path.Join(dir, filename),
	}

}