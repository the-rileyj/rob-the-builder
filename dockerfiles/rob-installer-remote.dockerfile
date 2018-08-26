FROM golang:1.10.3-alpine3.8

WORKDIR /go/src/github.com/the-rileyj

RUN apk update && \
    apk upgrade && \
    apk add --no-cache gcc git musl-dev

ADD https://github.com/the-rileyj/rob-the-builder/archive/master.tar.gz ./

RUN tar -xzf master.tar.gz -C ./ && \
    mv ./rob-the-builder-master rob && \
    rm -rf master.tar.gz

RUN cd rob && \
    go get -d ./... && \
    env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
	go build --ldflags '-extldflags "-static"' \
	-o /bin/rob && \
    cd / && \
    ls | grep -v "^bin$\|dev\|etc\|home\|lib\|proc\|sys" | xargs rm -rf

WORKDIR /

ENTRYPOINT cat ./bin/rob