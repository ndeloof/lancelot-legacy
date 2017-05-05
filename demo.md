```
docker run -it --rm -v /var/run/docker.sock:/var/run/docker.sock --name lancelot -p 2379:2375 cloudbees/lancelot -v slave:/home/jenkins -p 2222:22 ndeloof/jenkins-ssh-slave-with-docker-cli  "ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEArRovnMARwkO/flo1fwRcySMd4mkbbi0RkN+X4hT8xcg+Z8qqV6sa5UcxH+B/sxKpRlwj6HpM/THRLVfKajKisG1RyGrN4RDAsWr7ZlGLNvdhg+12znJK9Ff+ZlByMs3AnFRu+WV5fUf9XjYm3lBDISaMy17LrjJ+Ck+d7zmpqYvcY75/9NLTY1CHwCfePV7Wo46SAFegaWmHqAvQf9gPzw5VDT9P97YH2S+vtU0ORT8pDDpOheWBWo6QaVz8oVxPDGi9MqhQElkEaQckg9WnGvle6b2v0SC4DFevWo3oIOPy6Zci1mHXi9K0L7J+2XBqTeh0fPRAWiKjDLnm9BaH6Q=="  
```

```
cbsupport-jenkins je 2.32.2.7
```