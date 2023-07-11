package server

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"gopkg.in/urfave/cli.v1"

	"github.com/octu0/example-envoy-xds"
)

func serverAction(c *cli.Context) error {
	initLogLevel(c)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	nodeId := c.String("node-id")
	xdsListenAddr := c.String("xds-listen-addr")
	alsListenAddr := c.String("als-listen-addr")

	if nodeId == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return err
		}
		nodeId = hostname
	}

	wf := xds.NewWatchFile(
		ctx,
		nodeId,
		xds.WatchCdsConfigFile(c.String("cds-yaml")),
		xds.WatchEdsConfigFile(c.String("eds-yaml")),
		xds.WatchRdsConfigFile(c.String("rds-yaml")),
		xds.WatchLdsConfigFile(c.String("lds-yaml")),
	)

	svr := xds.NewServer(
		ctx,
		wf.Cache(),
		xds.XdsListenAddr(xdsListenAddr),
		xds.AlsListenAddr(alsListenAddr),
	)

	log.Printf("info: server starting...")

	// initial load all files
	if err := wf.ReloadAll(); err != nil {
		log.Printf("error: load file(s) error: %s", err.Error())
		return err
	}
	if err := wf.Watch(ctx); err != nil {
		log.Printf("error: failed create to fsnotify: %s", err.Error())
		return err
	}

	go watchSignal(wf, cancel)

	go svr.Start()

	<-ctx.Done() // wait stop

	log.Printf("info: server stopping...")

	svr.Stop()

	log.Printf("info: server stop")
	return nil
}

func watchSignal(wf *xds.WatchFile, cancel context.CancelFunc) {
	trap := make(chan os.Signal)
	signal.Notify(trap, syscall.SIGTERM)
	signal.Notify(trap, syscall.SIGHUP)
	signal.Notify(trap, syscall.SIGQUIT)
	signal.Notify(trap, syscall.SIGINT)

	for {
		select {
		case sig := <-trap:
			log.Printf("info: signal trap(%s)", sig.String())
			switch sig {
			case syscall.SIGHUP:
				if err := wf.ReloadAll(); err != nil {
					log.Printf("warn: reload file(s) error: %s, skip update", err.Error())
				}
				continue

			case syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT:
				cancel()
				return
			}
		}
	}
}

func init() {
	addCommand(cli.Command{
		Name: "server",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:   "node-id",
				Usage:  "envoy node-id(must be the value specified in node.id in enovy.yaml)",
				Value:  "",
				EnvVar: "XDS_NODE_ID",
			},
			cli.StringFlag{
				Name:   "xds-listen-addr",
				Usage:  "grpc xds listen address",
				Value:  "[0.0.0.0]:8000",
				EnvVar: "XDS_LISTEN_ADDR",
			},
			cli.StringFlag{
				Name:   "als-listen-addr",
				Usage:  "grpc als listen address",
				Value:  "[0.0.0.0]:8001",
				EnvVar: "ALS_LISTEN_ADDR",
			},
			cli.StringFlag{
				Name:   "cds-yaml",
				Usage:  "/path/to/cds.yaml",
				Value:  "./cds.yaml",
				EnvVar: "CDS_YAML",
			},
			cli.StringFlag{
				Name:   "eds-yaml",
				Usage:  "/path/to/eds.yaml",
				Value:  "./eds.yaml",
				EnvVar: "EDS_YAML",
			},
			cli.StringFlag{
				Name:   "rds-yaml",
				Usage:  "/path/to/rds.yaml",
				Value:  "./rds.yaml",
				EnvVar: "RDS_YAML",
			},
			cli.StringFlag{
				Name:   "lds-yaml",
				Usage:  "/path/to/lds.yaml",
				Value:  "./lds.yaml",
				EnvVar: "LDS_YAML",
			},
		},
		Action: serverAction,
	})
}
