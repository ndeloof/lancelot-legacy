package proxy

import (
	"fmt"
	"net/http"
	"golang.org/x/net/context"
	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/runconfig"
	"github.com/gorilla/mux"
	"encoding/json"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)


func (p *Proxy) containerInspect(w http.ResponseWriter, r *http.Request) {

	name := mux.Vars(r)["name"]
	json, err := p.client.ContainerInspect(context.Background(), name)
	if err != nil {
		if client.IsErrContainerNotFound(err) {
			w.WriteHeader(http.StatusNotFound)
			return
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	httputils.WriteJSON(w, http.StatusOK, json) // TODO we could filter container by label to hide container created by another client
}


func (p *Proxy) containerCreate(w http.ResponseWriter, r *http.Request) {

	if err := httputils.ParseForm(r); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := httputils.CheckForJSON(r); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	name := r.Form.Get("name")

	decoder := runconfig.ContainerDecoder{}
	config, hostConfig, networkingConfig, err := decoder.DecodeConfig(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	httputils.WriteJSON(w, http.StatusCreated, &types.IDResponse{
		ID: body.ID,
	})
}

func (p *Proxy) containerStop(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]
	fmt.Println(name)
}

func (p *Proxy) containerExecCreate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]
	fmt.Println(name)

	if err := httputils.ParseForm(r); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := httputils.CheckForJSON(r); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	execConfig := &types.ExecConfig{}
	if err := json.NewDecoder(r.Body).Decode(execConfig); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if len(execConfig.Cmd) == 0 {
		http.Error(w, "No exec command specified", http.StatusBadRequest)
		return
	}

	// Register an instance of Exec in container.
	id, err := p.client.ContainerExecCreate(context.Background(), name, *execConfig)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	httputils.WriteJSON(w, http.StatusCreated, &types.IDResponse{
		ID: id.ID,
	})
}

func (p *Proxy) containerDelete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]
	fmt.Println(name)

	p.client.ContainerRemove(context.Background(), name, types.ContainerRemoveOptions{
		Force: httputils.BoolValue(r, "force"),
		RemoveVolumes: httputils.BoolValue(r, "v"),
		RemoveLinks: httputils.BoolValue(r, "link"),
	})
}

