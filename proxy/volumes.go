package proxy

import (
	"net/http"
	"github.com/docker/docker/api/server/httputils"
	"context"
	volumetypes "github.com/docker/docker/api/types/volume"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/gorilla/mux"
	"github.com/docker/docker/api/types/filters"
)

func (p *Proxy) volumeList(w http.ResponseWriter, r *http.Request) {

	if err := httputils.ParseForm(r); err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	filters, err := filters.FromParam(r.Form.Get("filters"))
	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	volumes, err := p.client.VolumeList(context.Background(), filters)
	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	filtered := []*types.Volume{}
	for _, v := range volumes.Volumes {
		if _, err := p.ownsVolume(v.Name); err == nil {
			filtered = append(filtered, v)
		}
	}

	httputils.WriteJSON(w, http.StatusOK, &volumetypes.VolumesListOKBody{
		Volumes: filtered,
		// TODO volumes.Warnings may leak informations on volumes we don't own
		// Maybe we should forge a filter with owned volumes
		Warnings: []string{},
	})


}

func (p *Proxy) volumeCreate(w http.ResponseWriter, r *http.Request) {

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

	var req volumetypes.VolumesCreateBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return

	}

	volume, err := p.client.VolumeCreate(context.Background(), volumetypes.VolumesCreateBody{
		Driver: req.Driver,
		DriverOpts: req.DriverOpts,
		Labels: req.Labels,
		Name: req.Name,
	})
	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	p.addVolume(volume.Name)
	httputils.WriteJSON(w, http.StatusCreated, volume)
}

func (p *Proxy) volumeDelete(w http.ResponseWriter, r *http.Request) {
	if err := httputils.ParseForm(r); err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(r)
	name, err := p.ownsVolume(vars["name"]);
	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	force := httputils.BoolValue(r, "force")
	if err := p.client.VolumeRemove(context.Background(), name, force); err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}