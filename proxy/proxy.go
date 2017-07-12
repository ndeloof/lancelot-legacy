package proxy

import (
	"github.com/docker/docker/client"
	"github.com/gorilla/mux"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/api"
	"fmt"
	"golang.org/x/net/context"
)

type Proxy struct {
	client client.APIClient
	cgroup string  // Our current cgroup, we will share with any container we run
	containers	[]string
	execs           []string
	images		[]string
}


// FIXME need to protect concurrent access to p.containers|execs|images
func (p *Proxy) addContainer(id string) {
	fmt.Printf("recording allowed access to container %s\n", id)
	p.containers = append(p.containers, id)
}

func (p *Proxy) addExec(id string) {
	p.execs = append(p.execs, id)
}

func (p *Proxy) addImage(id string) {
	fmt.Printf("recording allowed access to image %s\n", id)
	p.images = append(p.images, id)
}


func (p *Proxy) ownsContainer(id string) bool {
	return contains(p.containers, id)
}

func contains(list []string, value string) bool {
	for _,c := range list {
		if c == value {
			return true
		}
	}
	return false
}

func (p *Proxy) ownsExec(id string) bool {
	return contains(p.execs, id)
}

func (p *Proxy) ownsImage(id string) bool {
	return contains(p.images, id)
}

func (p *Proxy) RegisterRoutes(r *mux.Router) {
	r.Path("/_ping").Methods("GET").HandlerFunc(p.ping)
	r.Path("/v{version:[0-9.]+}/version").Methods("GET").HandlerFunc(p.version)
	r.Path("/v{version:[0-9.]+}/events").Methods("GET").HandlerFunc(p.events)

	r.Path("/v{version:[0-9.]+}/images/{name:.*}/json").Methods("GET").HandlerFunc(p.imageInspect)
	r.Path("/v{version:[0-9.]+}/images/create").Methods("POST").HandlerFunc(p.imagesCreate)

	r.Path("/v{version:[0-9.]+}/containers/json").Methods("GET").HandlerFunc(p.containerList)
	r.Path("/v{version:[0-9.]+}/containers/{name:.*}/json").Methods("GET").HandlerFunc(p.containerInspect)
	r.Path("/v{version:[0-9.]+}/containers/create").Methods("POST").HandlerFunc(p.containerCreate)
	r.Path("/v{version:[0-9.]+}/containers/{name:.*}/start").Methods("POST").HandlerFunc(p.containerStart)
	r.Path("/v{version:[0-9.]+}/containers/{name:.*}/resize").Methods("POST").HandlerFunc(p.containerResize)
	r.Path("/v{version:[0-9.]+}/containers/{name:.*}/stop").Methods("POST").HandlerFunc(p.containerStop)
	r.Path("/v{version:[0-9.]+}/containers/{name:.*}/exec").Methods("POST").HandlerFunc(p.containerExecCreate)
	r.Path("/v{version:[0-9.]+}/exec/{execId:.*}/start").Methods("POST").HandlerFunc(p.containerExecStart)
	r.Path("/v{version:[0-9.]+}/exec/{execId:.*}/resize").Methods("POST").HandlerFunc(p.containerExecResize)
	r.Path("/v{version:[0-9.]+}/exec/{execId:.*}/json").Methods("GET").HandlerFunc(p.execInspect)
	r.Path("/v{version:[0-9.]+}/containers/{name:.*}").Methods("DELETE").HandlerFunc(p.containerDelete)

	r.Path("/v{version:[0-9.]+}/build").Methods("POST").HandlerFunc(p.build)
}

func (p *Proxy) SetClient(c client.APIClient) {
	p.client = c

	i, err := p.client.Info(context.Background())
	if err != nil {
		panic(err)
	}
	fmt.Println("docker daemon running " + i.ServerVersion)

	ping, err := p.client.Ping(context.Background())
	if err != nil {
		panic(err)
	}

	if versions.LessThan(ping.APIVersion, api.DefaultVersion) {
		fmt.Printf("[[WARNING]] target docker daemon exposes API %s but proxy was designed for API version %s\n", ping.APIVersion, api.DefaultVersion)
	}
}

func (p *Proxy) SetCgroup(cgroup string) {
	p.cgroup = cgroup
}

func (p *Proxy) GetCgroup() string {
	return "/docker/" + p.cgroup
}




