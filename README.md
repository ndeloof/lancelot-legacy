# Lancelot, a docker API armor

![lancelot](lancelot.jpg)

Lancelot is a proxy for docker API to filter API calls and only let you use a minimal set of commands and options.
Main idea is to give you access to docker daemon from a container, but not give up with security and isolation.

Lancelot is developed with [docker-pipeline](https://wiki.jenkins-ci.org/display/JENKINS/CloudBees+Docker+Pipeline+Plugin) 
to drive Docker API usage, so it mostly focus on _only_ supporting the required API for docker-pipeline to support
`docker.inside`. More API might be supported in future for other usages, for example to support `docker.build`. 

## Why ?

### Why access docker API ?

When you run inside a container (let's say, to run a jenkins build) and at some point want to rdo something 
docker-related (for sample, build a new docker iamge from a `Dockerfile` or run a docker container).

Classic options are
* [Docker in Docker](https://github.com/jpetazzo/dind) which own creator tell you [NOT TO USE](https://jpetazzo.github.io/2015/09/03/do-not-use-docker-in-docker-for-ci/)
  ok, actually this _is_ used on [PWD](play-with-docker.com) so is not _so_ bad, but require some fine tuned infrastructure. 
* Side containers, i.e. run those other containers on the same Docker Host sharing volume / network / ... depending your use-case.

The later is the most recommended one, and require to grant access to the docker daemon. In most case on do bind mount 
the docker daemon socket `/var/run/docker.sock` inside build container, so it can run docker commands using the standard
docker API or cli.

### Impacts on security

BUT there's a major drawback doing this. Having access to docker socket, one become root on the host, breaking all
barriers set by docker containerization. Typically, running `docker run -v /:/target ubuntu bash` one can access
everything on host filesystem as root, without any restriction.

This is bad in general circumstance, but get worst when deployed on a cluster, as the node may host some cluster-wide
secret, like AWS access keys or Mesos API secrets. ** Doing this in such a cluster just throws away any security**

### Impacts on efficiency 
 
One can consider the cluster only host nice people who won't try to abuse permissions (sic). There's anyway another side
effect. Cluster is managed by an orchestrator responsible to find the best place to run tasks. So if you triggger a 
build with 2Gb requirement, your mesos / kubernetes / ecs / swarm cluster orchestrator will select a node with at least
2Gb memory available. Your container is configured with a 2Gb max memory limit (using Linux control groups). 
 
But if your container do run additional containers, they will also consume resources without your orchestrator to know 
about this. As a result, orchestrator may over-allocate a node because some container running there aren't created 
under its control.

This might result in unexpected build termination with Out of Memory Error.


## Proposed fix / workaround

Lancelot exposes docker API to containers but prevent use of dangerous APIs or parameters. Typically, it prevent running
privileged containers, bind mount from filesystem, etc. Filtering for API parameter allows to define fine grained white 
lists of allowed operations, to support a well defined set of use-cases.

Lancelot also force all sidecar containers to run within the _same_ cgroup as the main container. So if the orchestrator 
allocated 2Gb for a task, and configured cgroup with 2Gb memory limit, all containers will share this limit, and won't
be able to allocate more. 


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
   
   `docker run -t --link lancelot --env DOCKER_HOST=localhost:2375 -v slave:/home/jenkins -p 2222:22 ndeloof/jenkins-ssh-slave-with-docker-cli  "ssh-rsa ...=="`
 
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

### Filter accessible resources

Proxy is attached to a single client and as such can easily track all resources (containers, images) this client has created
on host. As a result we can block any attempt to access a container created by another user.
For images, we have no way to check if user has legitimate access to an image, or this one has been pulled / built
by another user with distinct credentials. So we have to try pulling from registry to check permissions.  

- [x] docker build (with parent cgroup inheritence)
- [x] docker run (with parent cgroup inheritence, bind mount prohibited)
- [x] docker ps (filtered)
- [x] docker inspect
- [x] docker exec
- [x] docker logs
- [x] docker cp
- [x] docker stop
- [x] docker images
- [x] docker image pull
- [x] docker image push
- [x] docker image inspect
- [x] docker tag
- [x] docker events 
- [x] docker info (minimal)
- [x] docker version
- [x] docker volumes create
- [x] docker volumes inspect
- [x] docker volumes ls (filtered)
- [x] docker volumes rm





### Internal details

Lancelot uses [docker/docker/client](https://github.com/docker/docker/tree/master/client) to access the actual docker
daemon.  

Server API implementation is moslty just copy/paste from [docker/docker-ce/api/server](https://github.com/docker/docker-ce/blob/master/components/engine/api/server/router).
as the [docker/docker-ce/api/types](https://github.com/docker/docker-ce/blob/master/components/engine/api/types) package is used both by client
and server, mapping data is trivial, but Lancelot do always create a new data struct to ensure we only allow parameter
we explicitly want to support (aka white-list).

Golang having no correct way to do this job (sic) dependencies are managed using [vndr](https://github.com/LK4D4/vndr). 
`vendor` directory is committed in repo anyway to make it easier checkout the code and quickly get it to run.

## Why this name
[Sir _Lancelot du Lac_](https://en.wikipedia.org/wiki/Lancelot) from Arthurian legend is both a great persona for 
armored security and an aquatic references required for anything related to Docker, isn't it ?

Lancelot also is a main character in French sitcom [Kaamelott](https://fr.wikipedia.org/wiki/Kaamelott), known for
being strict and inflexible.
 
 
