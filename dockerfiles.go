package main

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

const rootBuild string = `
FROM golang:alpine

ARG BUILD_NAME
ARG GOARCH
ARG GOOS

WORKDIR /app

ADD *.go .

RUN mkdir out
RUN apk update && apk upgrade && apk add --no-cache gcc git musl-dev
RUN go get -d ./...
RUN env CGO_ENABLED=0 GOOS=${GOOS} GOARCH=${GOARCH} \
	go build --ldflags '-extldflags "-static"' \
	-o ./out/${BUILD_NAME}`
