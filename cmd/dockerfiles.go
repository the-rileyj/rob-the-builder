package cmd

const localReactBuild string = `
FROM node:8.11.3-alpine

WORKDIR /app

COPY tslint.json .
COPY tsconfig.test.json .
COPY tsconfig.prod.json .
COPY tsconfig.json .
COPY package-lock.json .
COPY images.d.ts .
COPY package.json .

RUN npm install
ADD ./src ./src
ADD ./public ./public

ENTRYPOINT ["npm", "run", "build"]`

const remoteReactBuild string = `
FROM node:8.11.3-alpine

ARG GITHUB_URL
ARG GITHUB_DIR

ADD ${GITHUB_URL}/archive/master.tar.gz ./
RUN tar -xzf master.tar.gz -C ./ && mv ./${GITHUB_DIR}-master app

WORKDIR /app

RUN npm install

ENTRYPOINT ["npm", "run", "build"]`

const robInstallBuilderLocal string = `
FROM golang:1.10.3-alpine3.8

WORKDIR /go/src/github.com/the-rileyj/rob

RUN apk update && \
    apk upgrade && \
    apk add --no-cache gcc git musl-dev

ADD ./cmd ./cmd

COPY main.go .

RUN go get -d ./... && \
    env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
	go build --ldflags '-extldflags "-static"' \
	-o /bin/rob && \
    cd / && \
    ls | grep -v "^bin$\|dev\|etc\|home\|lib\|proc\|sys" | xargs rm -rf

WORKDIR /

ENTRYPOINT cat ./bin/rob`

const robInstallBuilderRemote string = `
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

ENTRYPOINT cat ./bin/rob`

const rootBuild string = `
FROM golang:1.10.3-alpine3.8

ARG BUILD_NAME
ARG GOARCH
ARG GOOS

WORKDIR /app

RUN mkdir out && mkdir dst && apk update && apk upgrade && apk add --no-cache gcc git musl-dev

ADD *.go .

RUN go get -d ./...
RUN env CGO_ENABLED=0 GOOS=${GOOS} GOARCH=${GOARCH} \
	go build --ldflags '-extldflags "-static"' \
	-o ./out/${BUILD_NAME}

ENV BUILD_NAME=${BUILD_NAME}

ENTRYPOINT cat ./out/${BUILD_NAME}`
