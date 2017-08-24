package proxy

import (
	"net/http"
	"golang.org/x/net/context"
	"github.com/docker/docker/api/server/httputils"
	"github.com/gorilla/mux"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/ioutils"
	"strings"
	"github.com/docker/docker/api/types"
	"io"
	"fmt"
	"github.com/docker/docker/api/types/filters"
)


func (p *Proxy) imagesList(w http.ResponseWriter, r *http.Request) {

	if err := httputils.ParseForm(r); err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	imageFilters, err := filters.FromParam(r.Form.Get("filters"))
	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	filterParam := r.Form.Get("filter")
	if filterParam != "" {
		imageFilters.Add("reference", filterParam)
	}

	images, err := p.client.ImageList(context.Background(), types.ImageListOptions{
		Filters: imageFilters,
		All: httputils.BoolValue(r, "all"),
	})
	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	filtered := []types.ImageSummary{}
	for _,i := range images {
		if p.ownsImage(i.ID) {
			filtered = append(filtered, i)
		}
	}

	httputils.WriteJSON(w, http.StatusOK, filtered)
}

func (p *Proxy) imageInspect(w http.ResponseWriter, r *http.Request) {

	name := mux.Vars(r)["name"]
	if !p.ownsImage(name) {
		w.WriteHeader(http.StatusNotFound)
		return
	}


	json, _, err := p.client.ImageInspectWithRaw(context.Background(), name)
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

func (p *Proxy) imagesCreate(w http.ResponseWriter, r *http.Request) {
	if err := httputils.ParseForm(r); err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	image := r.Form.Get("fromImage")


	if image == "" {
		http.Error(w, "Import is not supported", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	metaHeaders := map[string][]string{}
	for k, v := range r.Header {
		if strings.HasPrefix(k, "X-Meta-") {
			metaHeaders[k] = v
		}
	}

	authEncoded := r.Header.Get("X-Registry-Auth")
	reader, err := p.client.ImageCreate(context.Background(), image, types.ImageCreateOptions{
		RegistryAuth: authEncoded,
		
	})
	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	output := ioutils.NewWriteFlusher(w)
	defer output.Close()
	io.Copy(output, reader)


	// record both ID and all tags associated with image ID
	inspect, _, err := p.client.ImageInspectWithRaw(context.Background(), image)
	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	p.addImage(inspect.ID)
	for _, t := range inspect.RepoTags {
		p.addImage(t)
	}
}

func (p *Proxy) imageTag(w http.ResponseWriter, r *http.Request) {
	if err := httputils.ParseForm(r); err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	name := mux.Vars(r)["name"]
	if !p.ownsImage(name) {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	tag := r.Form.Get("tag")
	if err := p.client.ImageTag(context.Background(), name, tag); err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	p.addImage(tag)
	w.WriteHeader(http.StatusCreated)
}

func (p *Proxy) imagePush(w http.ResponseWriter, r *http.Request) {

	if err := httputils.ParseForm(r); err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tag := r.Form.Get("tag")
	name := mux.Vars(r)["name"]

	if tag != "" {
		// CLI is inconsistent and pass tag as a parameter, while "tag" du pass image:tag as URI path
		name = name+":"+tag
	}

	if !p.ownsImage(name) {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	authEncoded := r.Header.Get("X-Registry-Auth")
	reader, err := p.client.ImagePush(context.Background(), name, types.ImagePushOptions{
		RegistryAuth: authEncoded,
	})
	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	output := ioutils.NewWriteFlusher(w)
	defer output.Close()
	io.Copy(output, reader)
}