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
	"github.com/docker/cli/cli/command"
	c "github.com/docker/cli/cli/command/container"
	"github.com/docker/cli/cli/flags"
	"net"
)


func main() {

	fmt.Println(`
.____                               .__          __
|    |   _____    ____   ____  ____ |  |   _____/  |_
|    |   \__  \  /    \_/ ___\/ __ \|  |  /  _ \   __\
|    |___ / __ \|   |  \  \__\  ___/|  |_(  <_> )  |
|_______ (____  /___|  /\___  >___  >____/\____/|__|
        \/    \/     \/     \/    \/
        `)

	client, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}
	p := &proxy.Proxy{}
	p.SetClient(client)

	cgroup, err := selfContainerId()
	if err != nil {
		panic(err)
	}
	p.SetCgroup(cgroup)

	me, err := selfContainerName()
	if err != nil {
		panic(err)
	}
	p.SetHostname(me)

	// subscribe to SIGINT signals
	stopChan := make(chan os.Signal)
	signal.Notify(stopChan, os.Interrupt)

	m := mux.NewRouter()

	p.RegisterRoutes(m)

	loggedRouter := handlers.LoggingHandler(os.Stdout, m)

	srv := &http.Server{Addr: ":2375", Handler: loggedRouter}

	listener, err := net.Listen("tcp", ":2375")
	if err != nil {
		panic(err)
	}
	
	go srv.Serve(listener)
	fmt.Println("Lancelot Proxy started")


	args := os.Args[1:]
	fmt.Println(args)
	if len(args) > 0 {
		if err := runSidecarContainer(args, cgroup, me); err != nil {
			panic(err)
		}
	}


	<-stopChan // wait for SIGINT

	// shut down gracefully, but wait no longer than 5 seconds before halting
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	srv.Shutdown(ctx)

	p.Stop()
}

/**
 * start first sidecar container.
 * lancelot can receive the exact same arguments as a `docker run` command.
 */
func runSidecarContainer(args []string, cgroup string, lancelot string) error {

	// following code is mostly a copy paste from github.com/docker/docker/cmd/docker/docker.go:main()
	stdin, stdout, stderr := term.StdStreams()
	logrus.SetOutput(stderr)
	dockerCli := command.NewDockerCli(stdin, stdout, stderr)

	// Initialize CLI, configured to access docker socket directly
	dockerCli.Initialize(&flags.ClientOptions{
		Common: &flags.CommonOptions{
			Hosts: []string { "tcp://localhost:2375" },
			TLS: false,
		},
	})

	// Use a RunCommand to parse command line args
	cmd := c.NewRunCommand(dockerCli)

	// force new container to run within the same cgroup hierarchy
	args = append([]string{"--cgroup-parent", cgroup, "--link", lancelot, "--env", "DOCKER_HOST="+lancelot}, args...)

	fmt.Printf("Starting sidecar container %v\n", args)
	cmd.SetArgs(args)

	// run forrest, run
	return cmd.Execute()
}

/**
 * detect container ID when lancelot is deployed as a docker container
 * which should always be the case but for development
 */
func selfContainerId() (string, error) {

	inFile, _ := os.Open("/proc/self/cgroup")
	defer inFile.Close()

	pids := regexp.MustCompile(`^[0-9]+:pids:/docker/([0-9a-z]+)`)
	scanner := bufio.NewScanner(inFile)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		me := pids.FindStringSubmatch(scanner.Text())
		if len(me) > 0 {
			return me[1], nil
		}
	}
	return "", errors.New("not running inside a container")
}

func selfContainerName() (string, error) {
	return os.Hostname()
}
