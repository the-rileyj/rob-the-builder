FROM golang:alpine

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

CMD cat ./out/${BUILD_NAME}