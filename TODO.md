# TODO

Current implementation do copy/paste/adapt code from [docker server engine](https://github.com/docker/docker-ce/blob/master/components/engine/api/server/router/container/container_routes.go).
To reduce duplication / maintenance cost, it would be better to implement an alternative [backend](https://github.com/docker/docker-ce/blob/master/components/engine/api/server/router/container/backend.go)
and assemble a custom daemon to use it.
 
 