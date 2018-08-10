FROM golang:alpine AS build-env
ADD . /work
WORKDIR /work
RUN go build -o uniNpmCI main.go

FROM busybox
COPY --from=build-env /work/uniNpmCI /usr/local/bin/uniNpmCI
ENTRYPOINT ["/usr/local/bin/uniNpmCI"]