FROM golang:1.9 as build

COPY . $GOPATH/src/github.com/cloudbees/lancelot/
RUN go install github.com/cloudbees/lancelot

FROM golang:1.9
COPY --from=build /go/bin/lancelot /go/bin/lancelot
EXPOSE 2375
ENTRYPOINT ["/go/bin/lancelot"]