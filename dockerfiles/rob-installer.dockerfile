FROM golang:1.10.3-alpine3.8

WORKDIR /

ADD https://github.com/the-rileyj/rob-the-builder/archive/master.tar.gz ./

RUN apk update && \
    apk upgrade && \
    apk add --no-cache gcc git musl-dev && \
    tar -xzf master.tar.gz -C ./ && \
    mv ./rob-the-builder-master app && \
    rm -rf master.tar.gz && \
    cd app && \
    go get -d ./... && \
    env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
	go build --ldflags '-extldflags "-static"' \
	-o ../bin/rob && \
    cd / && \
    ls | grep -v "^bin$\|dev\|etc\|home\|lib\|proc\|sys" | xargs rm -rf

ENTRYPOINT cat ./bin/rob