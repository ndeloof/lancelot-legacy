package proxy

import (
	"net/http"
	"golang.org/x/net/context"
	"github.com/docker/docker/api/server/httputils"
	"github.com/gorilla/mux"
	"github.com/docker/docker/client"
)


func (p *Proxy) imageInspect(w http.ResponseWriter, r *http.Request) {

	name := mux.Vars(r)["name"]
	json, _, err := p.client.ImageInspectWithRaw(context.Background(), name)
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