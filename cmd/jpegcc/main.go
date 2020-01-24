// Management Console
package main

import (
	"context"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/regorov/jpegcc"
	"github.com/rs/zerolog"
	"github.com/urfave/cli"
)

// EnvVarPrefix holds environment variables prefix related to application.
const (
	EnvVarPrefix = "JPEGCC_"
)

func main() {

	app := cli.NewApp()
	app.Name = "jpegcc"
	app.Usage = "JPEG color counter"
	app.Version = BuildNumber
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:   "debug, d",
			Usage:  "debug mode activation",
			EnvVar: EnvVarPrefix + "DEBUG",
		},
		cli.StringFlag{
			Name:   "pl",
			Usage:  "pprof HTTP listener",
			EnvVar: EnvVarPrefix + "PPROF_LISTENER",
		},
	}

	app.Commands = []cli.Command{
		{
			Name:    "start",
			Aliases: []string{"s"},
			Usage:   "start application",

			Action: start,
			Flags: []cli.Flag{
				cli.IntFlag{
					Name:   "dworkers, dw",
					Value:  runtime.NumCPU(),
					Usage:  "amount of parallel download goroutines",
					EnvVar: EnvVarPrefix + "DWORKERS",
				},

				cli.IntFlag{
					Name:   "pworkers, pw",
					Value:  runtime.NumCPU(),
					Usage:  "amount of parallel image processing goroutines",
					EnvVar: EnvVarPrefix + "PWORKERS",
				},
				cli.StringFlag{
					Name:   "input, i",
					Value:  "input.txt",
					Usage:  "input file name",
					EnvVar: EnvVarPrefix + "INPUT",
				},
				cli.StringFlag{
					Name:   "output, o",
					Value:  "result.csv",
					Usage:  "output file name",
					EnvVar: EnvVarPrefix + "OUTPUT",
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		os.Exit(1)
	}
}

func start(c *cli.Context) error {

	debug := c.GlobalBool("debug")

	// 1. logger format preparation.
	zerolog.TimeFieldFormat = "20060102T150405.999Z07:00"
	zerolog.TimestampFieldName = "t"
	zerolog.MessageFieldName = "msg"
	zerolog.LevelFieldName = "lvl"

	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	logger.Info().Str("version", BuildNumber).Msg("application started")

	logger.Info().
		Bool("debug", debug).
		Str("input", c.String("input")).
		Str("output", c.String("output")).
		Int("pworkers", c.Int("pworkers")).
		Int("dworkers", c.Int("dworkers")).
		Msg("launching params")

	if debug {
		logger.Level(zerolog.DebugLevel)
	} else {
		logger.Level(zerolog.InfoLevel)
	}

	// 2. runtime profiling activation.
	if c.GlobalIsSet("pl") {
		go func(listen string) {
			logger.Info().Str("pl", listen).Msg("start pprof http listener")
			if err := http.ListenAndServe(listen, nil); err != nil {
				logger.Error().Str("errmsg", err.Error()).Msg("pprof listener starting failed")
			}
		}(c.GlobalString("pl"))
	}

	// 3. SIGINT capture.
	stop := make(chan os.Signal)
	signal.Notify(stop, syscall.SIGINT)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		<-stop
		logger.Info().Msg("signal SIGINT captured")
		cancel()
	}()

	// 4. Create objects.

	input := jpegcc.NewPlainTextFileInput(logger)
	downloader := jpegcc.NewMediaDownloader(logger, input)
	output := jpegcc.NewBufferedCSV(10)
	counter := jpegcc.NewCounterPix()
	imgproc := jpegcc.NewImageProcessor(logger, downloader, output, counter)

	if err := output.Open(c.String("output")); err != nil {
		logger.Error().Str("errmsg", err.Error()).Msg("output file open/create failed")
		return err
	}

	// 5. Start processes.
	downloader.Start(ctx, c.Int("dworkers"))

	if err := input.Start(ctx, c.String("input")); err != nil {
		logger.Error().Str("errmsg", err.Error()).Msg("input file open failed")
		return err
	}

	started := time.Now()
	logger.Info().Msg("processing started")
	imgproc.Start(ctx, c.Int("pworkers"))

	if err := output.Close(); err != nil {
		logger.Error().Str("errmsg", err.Error()).Msg("output file flush/close failed")
	}

	logger.Info().Str("dur", time.Since(started).String()).Msg("Completed")
	return nil
}
