FROM golang:1.8

COPY . $GOPATH/src/github.com/cloudbees/lancelot/
RUN go build github.com/cloudbees/lancelot

EXPOSE 2375
ENTRYPOINT ["/go/lancelot"]