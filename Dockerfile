FROM golang:1.8

COPY . $GOPATH/src/github.com/cloudbees/lancelot/
RUN cd $GOPATH/src/github.com/cloudbees/lancelot && go get && go build

EXPOSE 2375
ENTRYPOINT ['lancelot']