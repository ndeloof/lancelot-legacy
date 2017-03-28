# Lancelot, a docker API armor

![lancelot](lancelot.jpg)

Lancelot is a proxy for docker API to filter API calls and only let you use a minimal set of commands and options.
Main idea is to give you access to docker daemon from a container, but not give up with security and isolation.

## Implementation

Lancelot do expose docker API so it will look like a docker daemon but will forbid most APIs and will only let you run 
commands from white-list, as well as access resource from a validated set of paths. You typically can't bind mount `/`.

Lancelot uses [docker/docker/client](https://github.com/docker/docker/tree/master/client) to access the actual docker
daemon.  

## Why this name
_Lancelot du Lac_ is both a great persona for armored security and docker ecosystem aquatic references, isn't it ?
 
 