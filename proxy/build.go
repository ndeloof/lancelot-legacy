package proxy

import (
	"net/http"
	"golang.org/x/net/context"
	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/api/types"
	"io"
	"github.com/docker/go-units"
	"encoding/json"
	"strconv"
)


func (p *Proxy) build(w http.ResponseWriter, r *http.Request) {

	options := &types.ImageBuildOptions{
		Dockerfile: r.FormValue("dockerfile"),
		Tags: r.Form["t"],    // TODO support tag black/white-list

		SuppressOutput: httputils.BoolValue(r, "q"),
		NoCache: httputils.BoolValue(r, "nocache"),
		PullParent: httputils.BoolValue(r, "pull"),
		Squash: httputils.BoolValue(r, "squash"),
		ForceRemove: httputils.BoolValue(r, "forcerm"),
		Remove: httputils.BoolValue(r, "forcerm") || r.FormValue("rm") == "",

		// FIXME Those option should be set so intermediate containers get same constraint as caller
		MemorySwap: httputils.Int64ValueOrZero(r, "memswap"),
		Memory: httputils.Int64ValueOrZero(r, "memory"),
		CPUShares: httputils.Int64ValueOrZero(r, "cpushares"),
		CPUPeriod: httputils.Int64ValueOrZero(r, "cpuperiod"),
		CPUQuota: httputils.Int64ValueOrZero(r, "cpuquota"),
		CPUSetCPUs: r.FormValue("cpusetcpus"),
		CPUSetMems: r.FormValue("cpusetmems"),
		CgroupParent: p.GetCgroup(), // Force intermediate containers to use the same cgroup
		NetworkMode: r.FormValue("networkmode"),
	}

	if r.Form.Get("shmsize") != "" {
		shmSize, err := strconv.ParseInt(r.Form.Get("shmsize"), 10, 64)
		if err != nil {
			http.Error(w, "Import is not supported", http.StatusBadRequest)
			return
		}
		options.ShmSize = shmSize
	}

	var buildUlimits = []*units.Ulimit{}
	ulimitsJSON := r.FormValue("ulimits")
	if ulimitsJSON != "" {
		if err := json.Unmarshal([]byte(ulimitsJSON), &buildUlimits); err != nil {
			http.Error(w, "Import is not supported", http.StatusBadRequest)
			return
		}
		options.Ulimits = buildUlimits
	}

	var buildArgs = map[string]*string{}
	buildArgsJSON := r.FormValue("buildargs")

	if buildArgsJSON != "" {
		if err := json.Unmarshal([]byte(buildArgsJSON), &buildArgs); err != nil {
			http.Error(w, "Import is not supported", http.StatusBadRequest)
			return
		}
		options.BuildArgs = buildArgs
	}

	var labels = map[string]string{}
	labelsJSON := r.FormValue("labels")
	if labelsJSON != "" {
		if err := json.Unmarshal([]byte(labelsJSON), &labels); err != nil {
			http.Error(w, "Import is not supported", http.StatusBadRequest)
			return
		}
		options.Labels = labels
	}

	var cacheFrom = []string{}
	cacheFromJSON := r.FormValue("cachefrom")
	if cacheFromJSON != "" {
		if err := json.Unmarshal([]byte(cacheFromJSON), &cacheFrom); err != nil {
			http.Error(w, "Import is not supported", http.StatusBadRequest)
			return
		}
		options.CacheFrom = cacheFrom
	}
	
	res, err := p.client.ImageBuild(context.Background(), r.Body, *options)
	if err != nil {
		http.Error(w, "Import is not supported", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	output := ioutils.NewWriteFlusher(w)
	defer output.Close()
	io.Copy(output, res.Body)

}