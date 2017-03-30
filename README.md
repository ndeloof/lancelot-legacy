# Lancelot, a docker API armor

![lancelot](lancelot.jpg)

Lancelot is a proxy for docker API to filter API calls and only let you use a minimal set of commands and options.
Main idea is to give you access to docker daemon from a container, but not give up with security and isolation.

Lancelot is developed with [docker-pipeline](https://wiki.jenkins-ci.org/display/JENKINS/CloudBees+Docker+Pipeline+Plugin) 
to drive Docker API usage, so it mostly focus on _only_ supporting the required API for docker-pipeline to support
`docker.inside`. More API might be supported in future for other usages, for example to support `docker.build`. 

## Usage and Architecture

Lancelot is a standalone program to run on docker host and configure so it can access the docker daemon. It exposes
docker API one can bind to a build container. As a side effect, container doesn't need to run as root to access the API.
 
```
   +-----------------------+                                       +-----------------------+
   |  build container      | . . . . . . . . . . . . . . . . . . . | sidecar container     |
   |  docker client        |                                   ,-> |                       |
   |  DOCKER_HOST=tcp:2375 |                                   |   |                       |
   +--------------'--------+       +----------------+          |   +-----------------------+
                  '                | lancelot proxy |          |
                  '                |                |          |
                  '----(http)---------- lancelot ---------- dockerd  
                                   +----------------+
``` 
 
Lancelot itself can run as a container deployed with the build container, linked together.

1. start lancelot container, giving him access to underlying docker daemon :
 
   `docker run -it --name lancelot -v /var/run/docker.sock:/var/run/docker.sock cloudbees/lancelot`
 
1. start build container linked to lancelot and with `DOCKER_HOST` set so build container will use lancelot as docker API endpoint.

   `docker run --link lancelot --env DOCKER_HOST=localhost:2375 ... jenkins/some-slave-image` 
   
   for sample, I'm using a permanent agent connect via ssh on port 2222 to a dockerized ssh slave (with docker cli installed) :
   
   `docker run -t --link lancelot --env DOCKER_HOST=localhost:2375 -v /Users/nicolas/jenkins-slave:/home/jenkins -p 2222:22 ndeloof/jenkins-ssh-slave-with-docker-cli  "ssh-rsa ...=="`
 
1. enjoy 
 

## Implementation

Lancelot do expose docker API so it will look like a docker daemon but will forbid most APIs and will only let you run 
commands from white-list, as well as access resource from a validated set of paths. You typically can't bind mount `/`.

### Filter supported APIs

Lancelot implements Docker API as a http server, but only implement those API methods that are required for the use-cases
we support and we consider to be a safe usage of Docker. From this point of view this is comparable to other API proxies
who use regular expressions to filter API calls to daemon.

### Filter supported parameters

Lancelot do re-create all API objets to be passed to actual docker daemon, so we can filter parameters to only allow those
we consider safe. Typically it allows to pass `--volumes-from` to share a volume between containers, but will reject a bind mount.
It could also do some computation on parameters, for sample prevent reference to a container / image that is not included in
end-user namespace, so one can't override system container images.

### Internal details

Lancelot uses [docker/docker/client](https://github.com/docker/docker/tree/master/client) to access the actual docker
daemon.  

Server API implementation is moslty just copy/paste from [docker/docker/api/server](https://github.com/docker/docker/tree/master/api/server).
as the [docker/docker/api/types](https://github.com/docker/docker/tree/master/api/types) package is used both by client
and server, mapping data is trivial, but Lancelot do always create a new data struct to ensure we only allow parameter
we explicitly want to support (aka white-list).

Golang having no correct way to do this job (sic) dependencies are managed using [vndr](https://github.com/LK4D4/vndr). 
`vendor` directory is committed in repo anyway to make it easier checkout the code and quickly get it to run.

## Why this name
[Sir _Lancelot du Lac_](https://en.wikipedia.org/wiki/Lancelot) from Arthurian legend is both a great persona for 
armored security and an aquatic references required for anything related to Docker, isn't it ?

Lancelot also is a main character in French sitcom [Kaamelott](https://fr.wikipedia.org/wiki/Kaamelott), known for
being strict and inflexible.
 
 