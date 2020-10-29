package main

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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
	}

}

// printConfig read from cli to stdout
func printConfig() {
	fmt.Printf("Config:\n")
	fmt.Printf("  petasos-endpoint: %s\n", petasosURL.String())
	fmt.Printf("  talaria-endpoint: %s\n", publicTalariaURL.String())
	fmt.Printf("  fixed-scheme:     %s\n", *fixedScheme)
	fmt.Printf("  log:              %s\n", *logFormat)
	fmt.Printf("  log-level:        %s\n", *logLevel)
	fmt.Printf("\n\n")
}
