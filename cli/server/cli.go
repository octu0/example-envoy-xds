package server

import (
	"github.com/comail/colog"
	"gopkg.in/urfave/cli.v1"
)

var (
	commands = make([]cli.Command, 0)
)

func addCommand(command cli.Command) {
	commands = append(commands, command)
}

func Commands() []cli.Command {
	return commands
}

func initLogLevel(c *cli.Context) {
	if c.GlobalBool("debug") {
		colog.SetMinLevel(colog.LDebug)
		if c.GlobalBool("verbose") {
			colog.SetMinLevel(colog.LTrace)
		}
	}
}
