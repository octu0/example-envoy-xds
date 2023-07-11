package main

import (
	"log"
	"os"

	"github.com/comail/colog"
	"gopkg.in/urfave/cli.v1"

	"github.com/octu0/example-envoy-xds"
	"github.com/octu0/example-envoy-xds/cli/server"
)

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
	app.Commands = server.Commands()
	app.Flags = []cli.Flag{
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
		log.Fatalf("error: %+v", err)
	}
}
