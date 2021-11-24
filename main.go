package main

import (
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

const Usage = "mydokcer is a very simple self-implement docker by Ziyi Guo"

func main() {
	app := cli.NewApp()
	app.Name = "mydocker"
	app.Usage = Usage

	app.Commands = []*cli.Command{
		initCommand,   // docker init
		runCommand,    // docker run
		commitCommand, // docker commit
		listCommand,   // docker ps
		logCommand,    // docker log
		execCommand,   // docker exec
		stopCommand,   // docker stop
		removeCommand, //docker rm
	}

	app.Before = func(context *cli.Context) error {
		log.SetFormatter(&log.JSONFormatter{})
		log.SetOutput(os.Stdout)
		return nil
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
