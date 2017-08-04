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

or

```bash                         
docker run -it --rm -v /var/run/docker.sock:/var/run/docker.sock --name lancelot -p 2379:2375 cloudbees/lancelot -d -v slave:/home/jenkins openjdk:8 bash -c "curl -fsSL https://get.docker.com/builds/Linux/x86_64/docker-1.13.1.tgz | tar --strip-components=1 -xz -C /usr/local/bin docker/docker && curl -v --connect-timeout 20  --max-time 60 -o slave.jar http://192.168.1.15:8080/jnlpJars/slave.jar && java -jar slave.jar -jnlpUrl http://192.168.1.15:8080/computer/docker-slave/slave-agent.jnlp -secret f2a02a7de9fe19c05bed0c6e1a3e354e5ff9f5a5ea7a06abd612e1134bb8b98a"

```


configure a jenkins ssh slave to connect into this container
run various docker related builds ...