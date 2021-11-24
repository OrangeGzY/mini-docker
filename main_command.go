package main

import (
	"fmt"
	"os"

	"./container"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var removeCommand = &cli.Command{
	Name:  "rm",
	Usage: "remove a stopped container",
	Action: func(context *cli.Context) error {
		if context.NArg() < 1 {
			return fmt.Errorf("Missing container's Name ")
		}
		containerName := context.Args().Get(0)
		removeContainer(containerName)
		return nil
	},
}

var stopCommand = &cli.Command{
	Name:  "stop",
	Usage: "stop a contianer",
	Action: func(context *cli.Context) error {
		if context.NArg() < 1 {
			return fmt.Errorf("Missing container's Name ")
		}
		containerName := context.Args().Get(0)
		stopContainer(containerName)
		return nil
	},
}

// mydocker exec
var execCommand = &cli.Command{
	Name:  "exec",
	Usage: "exec a command into container",
	Action: func(context *cli.Context) error {
		// For callback
		// The second time we will enter the if branch which means the env has been set and the Cgo code has been executed
		if os.Getenv(ENV_EXEC_PID) != "" {
			log.Infof("pid callback pid %d", os.Getgid())
			return nil
		}

		// The first time we will pass the if, and try to exec the Cgo code
		if context.NArg() < 2 {
			return fmt.Errorf("Misssing container name or command")
		}
		containerName := context.Args().Get(0)
		var commandArray []string
		for _, arg := range context.Args().Tail() {
			commandArray = append(commandArray, arg)
		}
		ExecContainer(containerName, commandArray)
		return nil
	},
}

// mydocker log
var logCommand = &cli.Command{
	Name:  "log",
	Usage: "Show the log of containers",
	Action: func(context *cli.Context) error {
		if context.NArg() < 1 {
			return fmt.Errorf("Please Input your container's Name")
		}
		containerName := context.Args().Get(0)
		logContianer(containerName)
		return nil
	},
}

// mydocker init
var initCommand = &cli.Command{
	Name: "init",

	Usage: "Init container process and run user's process in container. Do not call it outside",

	//获取command 容器初始化
	Action: func(context *cli.Context) error {
		// log.Infof("init")
		// cmd := context.Args().Get(0)
		// log.Infof("Command: %s", cmd)
		log.Infof("******* Container Initializing *******")
		err := container.RunContainerInitProcess()
		return err
	},
}

var listCommand = &cli.Command{
	Name:  "ps",
	Usage: "List all the container",
	Action: func(context *cli.Context) error {
		ListContainers()
		return nil
	},
}

// mydocker run
var runCommand = &cli.Command{
	Name: "run",
	Usage: `Create a container with namespace and cgroups limit
			mydocker run -ti [command]`,

	/*
		目前的命令
		-ti
		-m
		-cpushare
		-cpuset
	*/
	Flags: []cli.Flag{
		&cli.StringSliceFlag{
			Name:  "e",
			Usage: "set environment",
		},
		// -ti tty
		&cli.BoolFlag{
			Name:  "ti",
			Usage: "enable tty",
		},
		// -v volume
		&cli.StringFlag{
			Name:  "v",
			Usage: "volume",
		},
		// -d backend running mode
		&cli.BoolFlag{
			Name:  "d",
			Usage: "detach container",
		},
		// -name Specify the container's name
		&cli.StringFlag{
			Name:  "name ",
			Usage: "container name ",
		},
		&cli.StringFlag{
			Name:  "m",
			Usage: "memory limit",
		},
		&cli.StringFlag{
			Name:  "cpushare",
			Usage: "cpushare limit",
		},
		&cli.StringFlag{
			Name:  "cpuset",
			Usage: "cpuset limit",
		},
	},
	Action: func(context *cli.Context) error {
		// 检查run时的参数个数
		if context.NArg() < 1 {
			return fmt.Errorf("[runCommand] Missing container command ??????")
		}

		args := context.Args()
		cmdArray := make([]string, args.Len()) // command
		for index, cmd := range args.Slice() {
			cmdArray[index] = cmd
		}

		// Get the image name
		imageName := cmdArray[0]
		cmdArray = cmdArray[1:]
		// 检验是否使用tty交互模式
		createTty := context.Bool("ti")
		// Check if we use detach mode, container will run in the backend
		detach := context.Bool("d")
		// Check if -v is set
		volume := context.String("v")

		// Get the name of container
		contianerName := context.String("name")

		//
		envSlice := context.StringSlice("e")

		/*

			Note that, -d and -t cannot exist together !

		*/
		if createTty && detach {
			return fmt.Errorf("!!!!   -d and -t cannot set together   !!!!")
		}

		/*
			resConf := &subsystems.ResourceConfig{
				MemoryLimit: context.String("m"),
				CpuSet:      context.String("cpuset"),
				CpuShare:    context.String("cpushare"),
			}

			Run(tty, cmdArray, resConf)
		*/

		//log.Infof("createTty %v", createTty)
		Run(createTty, cmdArray, volume, contianerName, imageName, envSlice)
		return nil
	},
}

// mydocker commit
var commitCommand = &cli.Command{
	Name: "commit",

	Usage: "Commit a contaienr to image",

	//获取command 容器初始化
	Action: func(context *cli.Context) error {
		if context.NArg() < 2 {
			return fmt.Errorf("Missing container name ")
		}

		containerName := context.Args().Get(0)

		imageName := context.Args().Get(1)

		commitContainer(containerName, imageName)
		return nil
	},
}
