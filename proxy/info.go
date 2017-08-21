package proxy

import (
	"github.com/docker/docker/api"
	"net/http"
	"golang.org/x/net/context"
	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/pkg/ioutils"
	"encoding/json"
	"fmt"
)

func (p *Proxy) ping(w http.ResponseWriter, r *http.Request) {
	_, err := p.client.Ping(context.Background())
	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	httputils.WriteJSON(w, http.StatusOK, &types.Ping{
		APIVersion: api.DefaultVersion, // This is the API version we implement
	})
}

func (p *Proxy) version(w http.ResponseWriter, r *http.Request) {

	version, err := p.client.ServerVersion(context.Background())
	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Shall we filter output ?
	httputils.WriteJSON(w, http.StatusOK, version)
}

func (p *Proxy) info(w http.ResponseWriter, r *http.Request) {
	info, err := p.client.Info(context.Background())
	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	httputils.WriteJSON(w, http.StatusOK,
	// not sure which info are actually useful/required and which ones can leak host config and break security
	// so let just return a minimal set
	types.Info {
		ServerVersion: info.ServerVersion, // or proxy API version ?
		ID: info.ID,
		Architecture: info.Architecture,
		Containers: len(p.containers),
		IndexServerAddress: info.IndexServerAddress,
	})
}

func (p *Proxy) events(w http.ResponseWriter, r *http.Request) {
	if err := httputils.ParseForm(r); err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	since := r.Form.Get("since")
	until := r.Form.Get("until")
	args, err := filters.FromParam(r.Form.Get("filters"))
	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	msg, error := p.client.Events(context.Background(), types.EventsOptions{
		Since: since,
		Until: until,
		Filters: args,
	})

	w.Header().Set("Content-Type", "application/json")
	output := ioutils.NewWriteFlusher(w)
	output.Flush()
	enc := json.NewEncoder(output)
	for {
		select {
		case ev := <-msg:
			if err := enc.Encode(ev); err != nil {
				fmt.Println(err.Error())
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		case e := <-error:
			fmt.Println(err.Error())
			http.Error(w, e.Error(), http.StatusInternalServerError)
			return
		}
	}
}

