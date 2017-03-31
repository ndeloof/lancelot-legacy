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
)

func main() {

	client, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}
	p := &proxy.Proxy{}
	p.SetClient(client)


	inFile, _ := os.Open("/proc/self/cgroup")
	defer inFile.Close()

	pids := regexp.MustCompile(`^[0-9]+:pids:/docker/([0-9a-z]+)`)
	scanner := bufio.NewScanner(inFile)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		cgroup := pids.FindString(scanner.Text())
		if cgroup != "" {
			p.SetCGroup(cgroup)
		}

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

	<-stopChan // wait for SIGINT
	// shut down gracefully, but wait no longer than 5 seconds before halting
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	srv.Shutdown(ctx)
}
