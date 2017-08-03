Build lancelot container
```bash
docker build -t cloudbees/lancelot .
```

Run lancelot container
```bash
docker run -it --rm -v /var/run/docker.sock:/var/run/docker.sock --name lancelot -p 2379:2375 cloudbees/lancelot"  
```

configure your docker client to use Lancelot as docker host
```
export DOCKER_HOST=tcp://localhost:2379
```

run variosu docker commands, and check lancelot logs for proxied requests. 
```bash

docker version
docker ps
docker run -d -t ubuntu cat
docker stop $ID
docker kill $ID



```



# Run with a side container 
Add to lancelot launch command parameters for a classic `docker run` command, typically to launch a jenkins agent:

```bash
docker run -it --rm -v /var/run/docker.sock:/var/run/docker.sock --name lancelot -p 2379:2375 cloudbees/lancelot -v slave:/home/jenkins -p 2222:22 ndeloof/jenkins-ssh-slave-with-docker-cli  "ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEArRovnMARwkO/flo1fwRcySMd4mkbbi0RkN+X4hT8xcg+Z8qqV6sa5UcxH+B/sxKpRlwj6HpM/THRLVfKajKisG1RyGrN4RDAsWr7ZlGLNvdhg+12znJK9Ff+ZlByMs3AnFRu+WV5fUf9XjYm3lBDISaMy17LrjJ+Ck+d7zmpqYvcY75/9NLTY1CHwCfePV7Wo46SAFegaWmHqAvQf9gPzw5VDT9P97YH2S+vtU0ORT8pDDpOheWBWo6QaVz8oVxPDGi9MqhQElkEaQckg9WnGvle6b2v0SC4DFevWo3oIOPy6Zci1mHXi9K0L7J+2XBqTeh0fPRAWiKjDLnm9BaH6Q=="  
```
