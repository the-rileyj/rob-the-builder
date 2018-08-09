# escape=`

# Compiles *.go program in ./app,
# then transfers the compiled go program and
# all non-*.go files in ./app to /app in the container

# NOTE: golang:alpine sets WORKDIR to /go,
# which is why /go/gogram is used to copy the program
# instead of just /gogram
FROM golang:alpine AS buildenv

# Add gcc and musl-dev for any cgo dependencies, and
# git for getting dependencies residing on github
RUN apk add --no-cache gcc git musl-dev
# WORKDIR /app
ADD ./app/*.go .
# Get dependencies locally, but don't install
RUN go get -d ./...
# Compile program with local dependencies
RUN go build -o gogram

#env CGO_ENABLED=0 GOOS="linux" GOARCH="amd64" go build --ldflags "-linkmode external -extldflags -static"
#env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a --ldflags '-linkmode external -extldflags "-static"' .
#http://blog.wrouesnel.com/articles/Totally%20static%20Go%20builds/

# Second stage of build, adding in files and running
# newly compiled program
FROM alpine

# Create and navigate into /app so that the files we
# bring in aren't cluttered with the dirs in /
WORKDIR /app

# Add HTTPS Certificates
RUN apk update && `
    apk add ca-certificates && `
    rm -rf /var/cache/apk/*
# Copy the *.go program compiled in the first stage
COPY --from=buildenv /go/gogram .
# Copy the files on our machine in ./app to the container
COPY ./app .
# Remove extra unnessesary *.go files
RUN rm -r *.go

# Expose port 8080 to host machine
EXPOSE 8080
# Run program
ENTRYPOINT ./gogram