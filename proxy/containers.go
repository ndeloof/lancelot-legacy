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
	"io"
	"strconv"
	"net"
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
		VolumesFrom: hostConfig.VolumesFrom,
	}, networkingConfig, name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	httputils.WriteJSON(w, http.StatusCreated, &types.IDResponse{
		ID: body.ID,
	})
}

func (p *Proxy) containerStart(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	p.client.ContainerStart(context.Background(), name, types.ContainerStartOptions{
	})
	w.WriteHeader(http.StatusNoContent)
}

func (p *Proxy) containerStop(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]
	fmt.Println(name)
	w.WriteHeader(http.StatusNoContent)
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

func (p *Proxy) containerExecResize(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	execId := vars["execId"]
	fmt.Println(execId)

	if err := httputils.ParseForm(r); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		fmt.Println(err.Error())
		return
	}
	height, err := strconv.Atoi(r.Form.Get("h"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		fmt.Println(err.Error())
		return
	}
	width, err := strconv.Atoi(r.Form.Get("w"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		fmt.Println(err.Error())
		return
	}

	err = p.client.ContainerExecResize(context.Background(), execId, types.ResizeOptions{
		Height: uint(height),
		Width: uint(width),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		fmt.Println(err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (p *Proxy) containerExecStart(w http.ResponseWriter, r *http.Request) {
	execId := mux.Vars(r)["execId"]
	execStartCheck := &types.ExecStartCheck{}
	if err := json.NewDecoder(r.Body).Decode(execStartCheck); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}


	if execStartCheck.Detach {
		http.Error(w, "sorry we only support attached mode", http.StatusInternalServerError)
		return
	}

	resp, err := p.client.ContainerExecAttach(context.Background(), execId, types.ExecConfig{
		AttachStdin: false,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, stdout, err := httputils.HijackConnection(w)
	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, upgrade := r.Header["Upgrade"]
	if upgrade {
		fmt.Fprintf(stdout, "HTTP/1.1 101 UPGRADED\r\nContent-Type: application/vnd.docker.raw-stream\r\nConnection: Upgrade\r\nUpgrade: tcp\r\n\r\n")
	} else {
		fmt.Fprintf(stdout, "HTTP/1.1 200 OK\r\nContent-Type: application/vnd.docker.raw-stream\r\n\r\n")
	}

	// Pipe
	go func() {
		defer resp.Close()
		defer stdout.(net.Conn).Close()
		b, err := io.Copy(stdout, resp.Reader)
		if err != nil {
			fmt.Println("ouch...")
			fmt.Println(err.Error())
		}
		fmt.Printf("End of stdout, %d bytes written\n", b)
	}()
}


func (p *Proxy) execInspect(w http.ResponseWriter, r *http.Request) {

	execId := mux.Vars(r)["execId"]
	json, err := p.client.ContainerExecInspect(context.Background(), execId)
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

