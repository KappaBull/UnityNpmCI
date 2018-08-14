FROM golang:alpine AS build-env
ADD . /go/src/uniNpmCI
WORKDIR /go/src/uniNpmCI

#dep
RUN apk update
RUN apk add --no-cache git
RUN go get -u github.com/golang/dep/cmd/dep
RUN dep ensure

RUN go build -o uniNpmCI main.go

FROM alpine
RUN apk --update add git openssh && \
    rm -rf /var/lib/apt/lists/* && \
    rm /var/cache/apk/*
COPY --from=build-env /go/src/uniNpmCI/uniNpmCI /usr/local/bin/uniNpmCI