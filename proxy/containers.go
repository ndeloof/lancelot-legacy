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
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/filters"
	"time"
	"github.com/docker/docker/pkg/ioutils"
	"encoding/base64"
)


func (p *Proxy) containerList(w http.ResponseWriter, r *http.Request) {

	if err := httputils.ParseForm(r); err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	filter, err := filters.FromParam(r.Form.Get("filters"))
	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	config := types.ContainerListOptions{
		All:     httputils.BoolValue(r, "all"),
		Size:    httputils.BoolValue(r, "size"),
		Since:   r.Form.Get("since"),
		Before:  r.Form.Get("before"),
		Filters: filter,
	}

	containers, err := p.client.ContainerList(context.Background(), config)
	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// NOTE as an alternative, we could also add a label to every container we create, and force a filter here
	mine := []types.Container{}
	for _, c := range containers {
		_, err := p.ownsContainer(c.ID);
		if err == nil {
			mine = append(mine, c)
		}
	}

	httputils.WriteJSON(w, http.StatusOK, mine)
}

func (p *Proxy) containerInspect(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	name, err := p.ownsContainer(vars["name"]);
	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json, err := p.client.ContainerInspect(context.Background(), name)
	if err != nil {
		if client.IsErrContainerNotFound(err) {
			w.WriteHeader(http.StatusNotFound)
			return
		} else {
			fmt.Println(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	httputils.WriteJSON(w, http.StatusOK, json) // TODO we could filter container by label to hide container created by another client
}


func (p *Proxy) containerCreate(w http.ResponseWriter, r *http.Request) {

	if err := httputils.ParseForm(r); err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := httputils.CheckForJSON(r); err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	name := r.Form.Get("name")

	decoder := runconfig.ContainerDecoder{}
	config, hostConfig, networkingConfig, err := decoder.DecodeConfig(r.Body)
	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Binds is the old API
	binds := hostConfig.Binds
	for _, b := range binds {
		if b[:1] == "/" {
			http.Error(w, "Bind mount are not authorized", http.StatusUnauthorized)
			return
		}
	}

	// Mounts is the new API with explicit types
	mounts := hostConfig.Mounts
	for _, m := range mounts {
		if m.Type == mount.TypeBind {
			http.Error(w, "Bind mount are not authorized", http.StatusUnauthorized)
			return
		}
	}

	volumesFrom := []string{}
	for _, c := range hostConfig.VolumesFrom {
		id, err := p.ownsContainer(c)
		if err != nil {
			fmt.Println(err.Error())
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		volumesFrom = append(volumesFrom, id)
	}

	links := []string{}
	for _, c := range hostConfig.Links {
		id, err := p.ownsContainer(c)
		if err != nil && c != p.GetHostname() {
			fmt.Println(err.Error())
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		links = append(links, id)
	}

	auth := r.Header.Get("X-Registry-Auth")

	if !p.ownsImage(config.Image) {
		fmt.Printf("Checking legitimate access to image '%s' with credentials: %s\n", config.Image, auth);

		// We need to pull the image from registry to check client authentication let him access it
		load, err := p.client.ImagePull(context.Background(), config.Image, types.ImagePullOptions{
			All: false,
			RegistryAuth: auth,
		})
		// we pull to check permission, not to actually update image
		load.Close()
		if err != nil {
			fmt.Println(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		p.addImage(config.Image)
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
		Binds: binds,
		Mounts: mounts,
		Links: links,
		VolumesFrom: volumesFrom,
		Cgroup: container.CgroupSpec(p.GetCgroup()), // Force container to run within the same CGroup
	}, networkingConfig, name)
	if err != nil {
		if client.IsErrImageNotFound(err) {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			fmt.Println(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	p.addContainer(body.ID)
	if name != "" {
		// FIXME as we support named containers there's a risk for name collision
		// maybe force a prefix for names ?
		p.addContainer(name);
	}

	json, err := p.client.ContainerInspect(context.Background(), body.ID)
	if err := httputils.CheckForJSON(r); err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, m := range json.Mounts {
		if m.Type == mount.TypeVolume {
			p.addVolume(m.Name)
		}
	}

	httputils.WriteJSON(w, http.StatusCreated, &types.IDResponse{
		ID: body.ID,
	})
}

func (p *Proxy) containerStart(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name, err := p.ownsContainer(vars["name"]);
	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	p.client.ContainerStart(context.Background(), name, types.ContainerStartOptions{
	})
	w.WriteHeader(http.StatusNoContent)
}

// inspired by https://github.com/docker/docker-ce/blob/master/components/engine/api/server/router/container/container_routes.go #postContainersAttach
func (p *Proxy) containerAttach(w http.ResponseWriter, r *http.Request) {

	err := httputils.ParseForm(r)
	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	vars := mux.Vars(r)
	name, err := p.ownsContainer(vars["name"]);
	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	hijack, err := p.client.ContainerAttach(context.Background(), name, types.ContainerAttachOptions{
		Stdin:   httputils.BoolValue(r, "stdin"),
		Stdout:  httputils.BoolValue(r, "stdout"),
		Stderr:  httputils.BoolValue(r, "stderr"),
		Logs:       httputils.BoolValue(r, "logs"),
		Stream:     httputils.BoolValue(r, "stream"),
		DetachKeys: r.FormValue("detachKeys"),
	})
	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	p.hijack(w, hijack, r)
}

func (p *Proxy) hijack(w http.ResponseWriter, h types.HijackedResponse, r *http.Request) {

	stdin, stdout, err := httputils.HijackConnection(w)
	if err != nil {
		fmt.Println(err.Error())
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
		defer h.Close()
		defer stdout.(net.Conn).Close()
		b, err := io.Copy(stdout, h.Reader)
		if err != nil {
			fmt.Println("ouch...")
			fmt.Println(err.Error())
		}
		fmt.Printf("End of stdout, %d bytes written\n", b)
	}()

	go func() {
		defer stdin.(net.Conn).Close()
		b, err := io.Copy(h.Conn, stdin)
		if err != nil {
			fmt.Println("ouch...")
			fmt.Println(err.Error())
		}
		fmt.Printf("End of stdout, %d bytes written\n", b)
	}()
}

func (p *Proxy) containerResize(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name, err := p.ownsContainer(vars["name"]);
	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := httputils.ParseForm(r); err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	height, err := strconv.Atoi(r.Form.Get("h"))
	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	width, err := strconv.Atoi(r.Form.Get("w"))
	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = p.client.ContainerResize(context.Background(), name, types.ResizeOptions{
		Height: uint(height),
		Width: uint(width),
	})
	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (p *Proxy) containerLogs(w http.ResponseWriter, r *http.Request) {

	/* TODO
	vars := mux.Vars(r)
	name, err := p.ownsContainer(vars["name"]);
	if err != nil {
		fmt.Println(err.Error())		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	stdout, stderr := httputils.BoolValue(r, "stdout"), httputils.BoolValue(r, "stderr")
	if !(stdout || stderr) {
		return fmt.Errorf("Bad parameters: you must choose at least one stream")
	}

	reader, err := p.client.ContainerLogs(context.Background(), name, types.ContainerLogsOptions{
		ShowStdout: stdout,
		ShowStderr: stderr,
		Since:      r.Form.Get("since"),
		Timestamps: httputils.BoolValue(r, "timestamps"),
		Follow:	    httputils.BoolValue(r, "follow"),
		Tail:       r.Form.Get("tail"),
		Details:    httputils.BoolValue(r, "details"),
	});

	if err != nil {
		fmt.Println(err.Error())		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	*/

}

func (p *Proxy) containerStop(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name, err := p.ownsContainer(vars["name"]);
	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}


	if err := httputils.ParseForm(r); err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var seconds time.Duration
	if tmpSeconds := r.Form.Get("t"); tmpSeconds != "" {
		valSeconds, err := strconv.Atoi(tmpSeconds)
		if err != nil {
			fmt.Println(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		seconds = time.Duration(valSeconds)
	}

	p.client.ContainerStop(context.Background(), name, &seconds)

	w.WriteHeader(http.StatusNoContent)
}


func (p *Proxy) containerKill(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name, err := p.ownsContainer(vars["name"]);
	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	signal := r.Form.Get("signal")

	p.client.ContainerKill(context.Background(), name, signal)

	w.WriteHeader(http.StatusNoContent)
}

func (p *Proxy) containerExecCreate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name, err := p.ownsContainer(vars["name"]);
	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := httputils.ParseForm(r); err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := httputils.CheckForJSON(r); err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	execConfig := &types.ExecConfig{}
	if err := json.NewDecoder(r.Body).Decode(execConfig); err != nil {
		fmt.Println(err.Error())
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
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	p.addExec(id.ID)

	httputils.WriteJSON(w, http.StatusCreated, &types.IDResponse{
		ID: id.ID,
	})
}

func (p *Proxy) containerExecResize(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	execId := vars["execId"]
	if !p.ownsExec(execId) {
		http.Error(w, "You don't own " + execId, http.StatusUnauthorized)
	}

	if err := httputils.ParseForm(r); err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	height, err := strconv.Atoi(r.Form.Get("h"))
	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	width, err := strconv.Atoi(r.Form.Get("w"))
	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = p.client.ContainerExecResize(context.Background(), execId, types.ResizeOptions{
		Height: uint(height),
		Width: uint(width),
	})
	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (p *Proxy) containerExecStart(w http.ResponseWriter, r *http.Request) {
	execId := mux.Vars(r)["execId"]
	if !p.ownsExec(execId) {
		http.Error(w, "You don't own " + execId, http.StatusUnauthorized)
	}

	execStartCheck := &types.ExecStartCheck{}
	if err := json.NewDecoder(r.Body).Decode(execStartCheck); err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}


	if execStartCheck.Detach {
		http.Error(w, "sorry we only support attached mode", http.StatusInternalServerError)
		return
	}

	hijack, err := p.client.ContainerExecAttach(context.Background(), execId, types.ExecConfig{
		AttachStdin: httputils.BoolValue(r, "stdin"),
		AttachStdout: httputils.BoolValue(r, "stdout"),
		AttachStderr: httputils.BoolValue(r, "stderr"),
	})
	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	p.hijack(w, hijack, r)
}


func (p *Proxy) execInspect(w http.ResponseWriter, r *http.Request) {

	execId := mux.Vars(r)["execId"]
	if !p.ownsExec(execId) {
		http.Error(w, "You don't own " + execId, http.StatusUnauthorized)
	}

	json, err := p.client.ContainerExecInspect(context.Background(), execId)
	if err != nil {
		if client.IsErrContainerNotFound(err) {
			w.WriteHeader(http.StatusNotFound)
			return
		} else {
			fmt.Println(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	httputils.WriteJSON(w, http.StatusOK, json) // TODO we could filter container by label to hide container created by another client
}

func (p *Proxy) containerDelete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name, err := p.ownsContainer(vars["name"])
	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	p.client.ContainerRemove(context.Background(), name, types.ContainerRemoveOptions{
		Force: httputils.BoolValue(r, "force"),
		RemoveVolumes: httputils.BoolValue(r, "v"),
		RemoveLinks: httputils.BoolValue(r, "link"),
	})
}
func (p *Proxy) containerArchiveGet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name, err := p.ownsContainer(vars["name"])
	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	v, err := httputils.ArchiveFormValues(r, vars)
	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	reader, stat, err := p.client.CopyFromContainer(context.Background(), name, v.Path)
	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	statJSON, err := json.Marshal(stat)
	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/x-tar")
	w.Header().Set("X-Docker-Container-Path-Stat", base64.StdEncoding.EncodeToString(statJSON))

	output := ioutils.NewWriteFlusher(w)
	defer output.Close()
	io.Copy(output, reader)


}     
