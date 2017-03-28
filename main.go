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
	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/runconfig"
	"golang.org/x/net/context"
	"github.com/docker/docker/api/types"
	"encoding/json"
	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types/container"
)

type proxy struct {
	client client.APIClient
}

func (p *proxy) ping(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte{'O', 'K'})
}

func (p *proxy) containerCreate(w http.ResponseWriter, r *http.Request) {

	if err := httputils.ParseForm(r); err != nil {
		panic(err)
	}
	if err := httputils.CheckForJSON(r); err != nil {
		panic(err)
	}

	name := r.Form.Get("name")

	decoder := runconfig.ContainerDecoder{}
	config, hostConfig, networkingConfig, err := decoder.DecodeConfig(r.Body)
	if err != nil {
		panic(err)
	}

	body, err := p.client.ContainerCreate(context.Background(), &container.Config {
		Tty: config.Tty,
		User: config.User, // block user = root ?
		Env: config.Env,
		Cmd: config.Cmd,
		AttachStdout: config.AttachStdout,
		AttachStdin: config.AttachStdin,
		AttachStderr: config.AttachStderr,
		ArgsEscaped: config.ArgsEscaped,
		Entrypoint: config.Entrypoint,
		Image: config.Image,
		Volumes: config.Volumes,
		WorkingDir: config.WorkingDir,
	}, &container.HostConfig{
		Privileged: false,
		AutoRemove: hostConfig.AutoRemove,
		Binds: nil, // prevent bind mount
	}, networkingConfig, name)
	if err != nil {
		panic(err)
	}
	httputils.WriteJSON(w, http.StatusCreated, &types.IDResponse{
		ID: body.ID,
	})
}

func (p *proxy) containerStop(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]
	fmt.Println(name)
}

func (p *proxy) containerExecCreate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]
	fmt.Println(name)

	if err := httputils.ParseForm(r); err != nil {
		panic(err)
	}
	if err := httputils.CheckForJSON(r); err != nil {
		panic(err)
	}

	execConfig := &types.ExecConfig{}
	if err := json.NewDecoder(r.Body).Decode(execConfig); err != nil {
		panic(err)
	}

	if len(execConfig.Cmd) == 0 {
		fmt.Errorf("No exec command specified")
		return
	}

	// Register an instance of Exec in container.
	id, err := p.client.ContainerExecCreate(context.Background(), name, *execConfig)
	if err != nil {
		logrus.Errorf("Error setting up exec command in container %s: %v", name, err)
		panic(err)
	}

	httputils.WriteJSON(w, http.StatusCreated, &types.IDResponse{
		ID: id.ID,
	})
}

func (p *proxy) containerDelete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]
	fmt.Println(name)

	p.client.ContainerRemove(context.Background(), name, types.ContainerRemoveOptions{
		Force: httputils.BoolValue(r, "force"),
		RemoveVolumes: httputils.BoolValue(r, "v"),
		RemoveLinks: httputils.BoolValue(r, "link"),
	})
}

func main() {

	client, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}
	i, err := client.Info(context.Background())
	if err != nil {
		panic(err)
	}
	fmt.Println("docker daemon running " + i.ServerVersion)

	p := &proxy{}
	p.client = client


	// subscribe to SIGINT signals
	stopChan := make(chan os.Signal)
	signal.Notify(stopChan, os.Interrupt)

	m := mux.NewRouter()
	m.Path("/_ping").Methods("GET").HandlerFunc(p.ping)
	m.Path("/v{version:[0-9.]+}/containers/create").Methods("POST").HandlerFunc(p.containerCreate)
	m.Path("/v{version:[0-9.]+}/containers/{name:.*}/stop").Methods("POST").HandlerFunc(p.containerStop)
	m.Path("/v{version:[0-9.]+}/containers/{name:.*}/exec").Methods("POST").HandlerFunc(p.containerExecCreate)
	m.Path("/v{version:[0-9.]+}/containers/{name:.*}").Methods("DELETE").HandlerFunc(p.containerDelete)

	loggedRouter := handlers.LoggingHandler(os.Stdout, m)
	// http.ListenAndServe(":1123", loggedRouter)


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

