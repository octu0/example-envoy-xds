package main

import (
	"context"
	"log"
	"os"
	"runtime"

	"github.com/comail/colog"
	"gopkg.in/urfave/cli.v1"

	"github.com/octu0/example-envoy-xds"
)

type Config struct {
	DataDir     string
	LogDir      string
	DebugMode   bool
	VerboseMode bool
	Procs       int
}

var (
	commands = make([]cli.Command, 0)
)

func addCommand(command cli.Command) {
	commands = append(commands, command)
}

func createConfig(c *cli.Context) Config {
	config := Config{
		DataDir:     c.GlobalString("data-dir"),
		LogDir:      c.GlobalString("log-dir"),
		DebugMode:   c.GlobalBool("debug"),
		VerboseMode: c.GlobalBool("verbose"),
		Procs:       c.GlobalInt("procs"),
	}
	if config.DebugMode {
		colog.SetMinLevel(colog.LDebug)
		if config.VerboseMode {
			colog.SetMinLevel(colog.LTrace)
		}
	}

	if config.Procs < 1 {
		config.Procs = 1
	}
	return config
}

func initProcs(config Config) error {
	runtime.GOMAXPROCS(config.Procs)
	return nil
}

func prepareAction(commandName string, c *cli.Context) (context.Context, error) {
	config := createConfig(c)

	if err := initProcs(config); err != nil {
		log.Printf("error: runtime procs initialization failure: %s", err.Error())
		return nil, err
	}

	log.Printf("info: starting %s.%s-%s", xds.AppName, commandName, xds.Version)

	ctx := context.Background()

	return ctx, nil
}

func main() {
	colog.SetDefaultLevel(colog.LDebug)
	colog.SetMinLevel(colog.LInfo)

	colog.SetFormatter(&colog.StdFormatter{
		Flag: log.Ldate | log.Ltime | log.Lshortfile,
	})
	colog.Register()

	app := cli.NewApp()
	app.Version = xds.Version
	app.Name = xds.AppName
	app.Usage = ""
	app.Commands = commands
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "log-dir",
			Usage:  "log output directory",
			Value:  "/tmp",
			EnvVar: "XDS_LOG_DIR",
		},
		cli.StringFlag{
			Name:   "data-dir",
			Usage:  "data directory",
			Value:  "./data",
			EnvVar: "XDS_DATA_DIR",
		},
		cli.IntFlag{
			Name:   "procs, P",
			Usage:  "attach cpu(s)",
			Value:  runtime.NumCPU(),
			EnvVar: "XDS_NUM_CPU",
		},
		cli.BoolFlag{
			Name:   "debug, d",
			Usage:  "debug mode",
			EnvVar: "XDS_DEBUG",
		},
		cli.BoolFlag{
			Name:   "verbose, V",
			Usage:  "verbose. more message",
			EnvVar: "XDS_VERBOSE",
		},
	}
	if err := app.Run(os.Args); err != nil {
		log.Printf("error: %s", err.Error())
		cli.OsExiter(1)
	}
}
