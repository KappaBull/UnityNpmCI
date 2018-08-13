FROM golang:alpine AS build-env
ADD . /go/src/uniNpmCI
WORKDIR /go/src/uniNpmCI
RUN go get -u github.com/golang/dep/cmd/dep
RUN dep ensure
RUN go build -o uniNpmCI main.go

FROM busybox
COPY --from=build-env /go/src/uniNpmCI/uniNpmCI /usr/local/bin/uniNpmCI
ENTRYPOINT ["/usr/local/bin/uniNpmCI"]