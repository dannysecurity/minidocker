package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/dannysecurity/minidocker/internal/container"
	"github.com/dannysecurity/minidocker/internal/image"
	"github.com/dannysecurity/minidocker/internal/log"
)

const usage = `minidocker — a minimal container runtime for learning

Usage:
  minidocker pull <image>          Download and store an image
  minidocker run <image> <cmd...>  Run a command in a new container
  minidocker ps                    List running containers
  minidocker inspect <id>          Show container metadata as JSON
  minidocker logs <id>             Show container logs
  minidocker stop <id>             Stop a running container
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	var err error
	switch os.Args[1] {
	case "pull":
		err = cmdPull(os.Args[2:])
	case "run":
		err = cmdRun(os.Args[2:])
	case "ps":
		err = cmdPs()
	case "inspect":
		err = cmdInspect(os.Args[2:])
	case "logs":
		err = cmdLogs(os.Args[2:])
	case "stop":
		err = cmdStop(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n%s", os.Args[1], usage)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "minidocker: %v\n", err)
		os.Exit(1)
	}
}

func cmdPull(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: minidocker pull <image>")
	}
	store := image.NewStore(image.DefaultRoot)
	return store.Pull(args[0])
}

func cmdRun(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: minidocker run <image> <command...>")
	}
	imageName := args[0]
	command := args[1:]

	store := image.NewStore(image.DefaultRoot)
	rootfs, err := store.RootfsPath(imageName)
	if err != nil {
		return err
	}

	logger, err := log.NewLogger(log.DefaultRoot)
	if err != nil {
		return err
	}

	rt := container.NewRuntime(container.DefaultRoot, logger)
	_, err = rt.Run(container.RunSpec{
		Image:   imageName,
		Rootfs:  rootfs,
		Command: command,
	})
	return err
}

func cmdPs() error {
	rt := container.NewRuntime(container.DefaultRoot, nil)
	containers, err := rt.List()
	if err != nil {
		return err
	}
	if len(containers) == 0 {
		fmt.Println("CONTAINER ID  IMAGE  STATUS  COMMAND")
		return nil
	}
	fmt.Println("CONTAINER ID  IMAGE  STATUS  COMMAND")
	for _, c := range containers {
		fmt.Printf("%-12s  %-20s  %-8s  %s\n", c.ID, c.Image, c.Status, c.Command)
	}
	return nil
}

func cmdInspect(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: minidocker inspect <container-id>")
	}
	rt := container.NewRuntime(container.DefaultRoot, nil)
	info, err := rt.Inspect(args[0])
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func cmdLogs(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: minidocker logs <container-id>")
	}
	logger, err := log.NewLogger(log.DefaultRoot)
	if err != nil {
		return err
	}
	out, err := logger.Read(args[0])
	if err != nil {
		return err
	}
	fmt.Print(string(out))
	return nil
}

func cmdStop(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: minidocker stop <container-id>")
	}
	rt := container.NewRuntime(container.DefaultRoot, nil)
	return rt.Stop(args[0])
}
