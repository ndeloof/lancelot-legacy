package main

import (
	"github.com/docker/docker/client"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"
	"github.com/gorilla/mux"
	"github.com/gorilla/handlers"
	"golang.org/x/net/context"
	"github.com/cloudbees/lancelot/proxy"
	"regexp"
	"bufio"
	"github.com/pkg/errors"
	"github.com/docker/docker/pkg/term"
	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/cli/command"
	c "github.com/docker/docker/cli/command/container"
	"github.com/docker/docker/cli/flags"
)


func main() {

	client, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}
	p := &proxy.Proxy{}
	p.SetClient(client)

	me, err := selfContainerId()
	if err == nil {
		p.SetCGroup("/docker/" + me)
	}


	// subscribe to SIGINT signals
	stopChan := make(chan os.Signal)
	signal.Notify(stopChan, os.Interrupt)

	m := mux.NewRouter()

	p.RegisterRoutes(m)

	loggedRouter := handlers.LoggingHandler(os.Stdout, m)

	srv := &http.Server{Addr: ":2375", Handler: loggedRouter}

	go func() {
		// service connections
		if err := srv.ListenAndServe(); err != nil {
			fmt.Printf("listen: %s\n", err)
		}
	}()


	args := os.Args[1:]
	if len(args) > 0 {
		if err := runSidecarContainer(args, me); err != nil {
			panic(err)
		}
	}


	<-stopChan // wait for SIGINT
	// shut down gracefully, but wait no longer than 5 seconds before halting
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	srv.Shutdown(ctx)
}

func runSidecarContainer(args []string, parent string) error {
	fmt.Printf("Starting sidecar container %v\n", args)
	stdin, stdout, stderr := term.StdStreams()
	logrus.SetOutput(stderr)

	dockerCli := command.NewDockerCli(stdin, stdout, stderr)

	dockerCli.Initialize(&flags.ClientOptions{
		Common: &flags.CommonOptions{
			Hosts: []string { "unix:///var/run/docker.sock" },
			TLS: false,
		},
	})

	cmd := c.NewRunCommand(dockerCli)

	if parent != "" {
		fmt.Printf("Force cgroup parent %s\n", parent)
		// force new container to run within the same cgroup hierarchy
		args = append([]string{"--cgroup-parent", parent}, args...)
	}
	cmd.SetArgs(args)

	return cmd.Execute()
}

func selfContainerId() (string, error) {

	inFile, _ := os.Open("/proc/self/cgroup")
	defer inFile.Close()

	pids := regexp.MustCompile(`^[0-9]+:pids:/docker/([0-9a-z]+)`)
	scanner := bufio.NewScanner(inFile)
	scanner.Split(bufio.ScanLines)
	var me string
	for scanner.Scan() {
		me = pids.FindString(scanner.Text())
		if me != "" {
			return me, nil
		}
	}
	return "", errors.New("not running inside a container")
}
