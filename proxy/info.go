package proxy

import (
	"github.com/docker/docker/api"
	"net/http"
	"golang.org/x/net/context"
	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/types"
)

func (p *Proxy) ping(w http.ResponseWriter, r *http.Request) {
	_, err := p.client.Ping(context.Background())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	httputils.WriteJSON(w, http.StatusOK, &types.Ping{
		APIVersion: api.DefaultVersion, // This is the API version we implement
	})
}

func (p *Proxy) info(w http.ResponseWriter, r *http.Request) {
	info, err := p.client.Info(context.Background())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	httputils.WriteJSON(w, http.StatusCreated, &types.Info{
		ID: info.ID,
		Isolation: "lancelot_du_lac",
		ServerVersion: api.DefaultVersion,
	})
}
