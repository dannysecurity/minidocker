package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/dannysecurity/minidocker/internal/container"
	"github.com/dannysecurity/minidocker/internal/image"
	"github.com/dannysecurity/minidocker/internal/log"
	"github.com/dannysecurity/minidocker/internal/network"
)

const usage = `minidocker — a minimal container runtime for learning

Usage:
  minidocker pull <image>              Download and store an image
  minidocker images                    List locally stored images
  minidocker run [-d] [-p host:container] <image> <cmd...>
                                       Run a command in a new container
  minidocker ps [-a] [-q]              List containers (running by default)
  minidocker inspect <id>              Show container metadata as JSON
  minidocker logs [--tail N] <id>      Show container logs
  minidocker exec <id> <cmd...>        Run a command in a running container
  minidocker stop <id>                 Stop a running container
  minidocker rm <id>                   Remove a stopped container
`

func main() {
	os.Exit(runCLI(os.Args))
}

func runCLI(args []string) int {
	if len(args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		return 1
	}

	var err error
	switch args[1] {
	case "pull":
		err = cmdPull(args[2:])
	case "images":
		err = cmdImages(args[2:])
	case "run":
		err = cmdRun(args[2:])
	case "ps":
		err = cmdPs(args[2:])
	case "inspect":
		err = cmdInspect(args[2:])
	case "logs":
		err = cmdLogs(args[2:])
	case "exec":
		err = cmdExec(args[2:])
	case "stop":
		err = cmdStop(args[2:])
	case "rm":
		err = cmdRm(args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n%s", args[1], usage)
		return 1
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "minidocker: %v\n", err)
		return 1
	}
	return 0
}

func cmdPull(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: minidocker pull <image>")
	}
	store := image.NewStore(image.DefaultRoot)
	return store.Pull(args[0])
}

func cmdImages(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("usage: minidocker images")
	}

	store := image.NewStore(image.DefaultRoot)
	images, err := store.List()
	if err != nil {
		return err
	}

	fmt.Println("REPOSITORY  DIGEST  SOURCE")
	for _, img := range images {
		source := img.Source
		if source == "" {
			source = "pull"
		}
		digest := img.Digest
		if len(digest) > 19 {
			digest = digest[:19] + "..."
		}
		fmt.Printf("%-20s  %-23s  %s\n", img.Name, digest, source)
	}
	return nil
}

func cmdRun(args []string) error {
	detach := false
	var portMappings []network.PortMapping
	var positional []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-d", "--detach":
			detach = true
		case "-p", "--publish":
			if i+1 >= len(args) {
				return fmt.Errorf("usage: minidocker run [-d] [-p host:container] <image> <command...>")
			}
			mapping, err := network.ParsePortMapping(args[i+1])
			if err != nil {
				return err
			}
			portMappings = append(portMappings, mapping)
			i++
		default:
			positional = append(positional, args[i])
		}
	}

	if len(positional) < 2 {
		return fmt.Errorf("usage: minidocker run [-d] [-p host:container] <image> <command...>")
	}
	imageName := positional[0]
	command := positional[1:]

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
		Image:        imageName,
		Rootfs:       rootfs,
		Command:      command,
		Detach:       detach,
		PortMappings: portMappings,
	})
	return err
}

func cmdPs(args []string) error {
	all := false
	quiet := false
	for _, arg := range args {
		switch arg {
		case "-a", "--all":
			all = true
		case "-q", "--quiet":
			quiet = true
		default:
			return fmt.Errorf("unknown flag for ps: %s", arg)
		}
	}

	rt := container.NewRuntime(container.DefaultRoot, nil)
	containers, err := rt.ListFiltered(all)
	if err != nil {
		return err
	}

	if !quiet {
		fmt.Println("CONTAINER ID  IMAGE  STATUS  COMMAND")
	}
	for _, c := range containers {
		if quiet {
			fmt.Println(c.ID)
		} else {
			fmt.Printf("%-12s  %-20s  %-8s  %s\n", c.ID, c.Image, c.Status, c.Command)
		}
	}
	return nil
}

func cmdInspect(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: minidocker inspect <container-id>")
	}
	rt := container.NewRuntime(container.DefaultRoot, nil)
	id, err := rt.ResolveID(args[0])
	if err != nil {
		return err
	}
	info, err := rt.Inspect(id)
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
	tail := 0
	var ids []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--tail":
			if i+1 >= len(args) {
				return fmt.Errorf("usage: minidocker logs [--tail N] <container-id>")
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil || n < 0 {
				return fmt.Errorf("--tail requires a non-negative integer")
			}
			tail = n
			i++
		default:
			ids = append(ids, args[i])
		}
	}

	if len(ids) != 1 {
		return fmt.Errorf("usage: minidocker logs [--tail N] <container-id>")
	}

	logger, err := log.NewLogger(log.DefaultRoot)
	if err != nil {
		return err
	}

	rt := container.NewRuntime(container.DefaultRoot, nil)
	id, err := rt.ResolveID(ids[0])
	if err != nil {
		return err
	}

	var out []byte
	if tail > 0 {
		out, err = logger.ReadTail(id, tail)
	} else {
		out, err = logger.Read(id)
	}
	if err != nil {
		return err
	}
	fmt.Print(string(out))
	return nil
}

func cmdExec(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: minidocker exec <container-id> <command...>")
	}
	rt := container.NewRuntime(container.DefaultRoot, nil)
	return rt.Exec(args[0], args[1:])
}

func cmdStop(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: minidocker stop <container-id>")
	}
	rt := container.NewRuntime(container.DefaultRoot, nil)
	id, err := rt.ResolveID(args[0])
	if err != nil {
		return err
	}
	return rt.Stop(id)
}

func cmdRm(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: minidocker rm <container-id>")
	}
	rt := container.NewRuntime(container.DefaultRoot, nil)
	return rt.Remove(args[0])
}
