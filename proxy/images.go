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
)


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

	p.addImage(image)

	output := ioutils.NewWriteFlusher(w)
	defer output.Close()
	io.Copy(output, reader)
}