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
#git&ssh
RUN apk --update add git openssh && \
    rm -rf /var/lib/apt/lists/* && \
    rm /var/cache/apk/* && \
    mkdir ~/.ssh && \
    chmod 700 ~/.ssh && \
    echo "Host github.com¥nStrictHostKeyChecking no¥nUserKnownHostsFile /dev/null¥nIdentityFile ~/.ssh/github_id_rsa" > ~/.ssh/config && \
    chmod 600

COPY --from=build-env /go/src/uniNpmCI/uniNpmCI /usr/local/bin/uniNpmCI
WORKDIR /usr/local/bin