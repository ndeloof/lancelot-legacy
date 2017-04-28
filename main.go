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
	if err != nil {
		panic(err)
	}
	p.SetCGroup("/docker/" + me)


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

/**
 * start first sidecar container.
 * lancelot can receive the exact same arguments as a `docker run` command.
 */
func runSidecarContainer(args []string, lancelot string) error {

	// following code is mostly a copy paste from github.com/docker/docker/cmd/docker/docker.go:main()
	stdin, stdout, stderr := term.StdStreams()
	logrus.SetOutput(stderr)
	dockerCli := command.NewDockerCli(stdin, stdout, stderr)

	// Initialize CLI, configured to access docker socket directly
	dockerCli.Initialize(&flags.ClientOptions{
		Common: &flags.CommonOptions{
			Hosts: []string { "unix:///var/run/docker.sock" },
			TLS: false,
		},
	})

	// Use a RunCommand to parse command line args
	cmd := c.NewRunCommand(dockerCli)

	// force new container to run within the same cgroup hierarchy
	args = append([]string{"--cgroup-parent", lancelot, "--link", lancelot, "--env", "DOCKER_HOST="+lancelot}, args...)

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
