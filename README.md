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
 
Lancelot itself can run as a container deployed with the build container, linked together, with `DOCKER_HOST` set so
build container will use lancelot as docker API endpoint.
 

## Implementation

Lancelot do expose docker API so it will look like a docker daemon but will forbid most APIs and will only let you run 
commands from white-list, as well as access resource from a validated set of paths. You typically can't bind mount `/`.

Lancelot uses [docker/docker/client](https://github.com/docker/docker/tree/master/client) to access the actual docker
daemon.  

## Why this name
[Sir _Lancelot du Lac_](https://en.wikipedia.org/wiki/Lancelot) from Arthurian legend is  
is both a great persona for armored security and an aquatic references required for anything related to Docker, isn't it ?
 
 