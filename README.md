# Lancelot, a docker API armor

![lancelot](lancelot.jpg)

Lancelot is a proxy for docker API to filter API calls and only let you use a minimal set of commands and options.
Main idea is to give you access to docker daemon from a container, but not give up with security and isolation.

Lancelot is developed with [docker-pipeline](https://wiki.jenkins-ci.org/display/JENKINS/CloudBees+Docker+Pipeline+Plugin) 
to drive Docker API usage, so it mostly focus on _only_ supporting the required API for docker-pipeline to support
`docker.inside`. More API might be supported in future for other usages, for example to support `docker.build`. 

## Implementation

Lancelot do expose docker API so it will look like a docker daemon but will forbid most APIs and will only let you run 
commands from white-list, as well as access resource from a validated set of paths. You typically can't bind mount `/`.

Lancelot uses [docker/docker/client](https://github.com/docker/docker/tree/master/client) to access the actual docker
daemon.  

## Why this name
[Sir _Lancelot du Lac_](https://en.wikipedia.org/wiki/Lancelot) from Arthurian legend is  
is both a great persona for armored security and an aquatic references required for anything related to Docker, isn't it ?
 
 