# Stage 0: Build #
# Use the official Golang image to create a build artifact.
# This is based on Debian and sets the GOPATH to /go.
FROM golang:1.19 as BUILDER

# Create and change to the app directory.
WORKDIR /app

# Retrieve application dependencies using go modules.
# Allows container builds to reuse downloaded dependencies.
COPY go.* ./
RUN go mod download

# Copy local code to the container image.
COPY . ./

# Build the binary.

# -mod=readonly: ensures immutable go.mod and go.sum in container builds.
# CGO_ENABLED=1: uses common libraries found on most major OS distributions.
# GOARCH=amd64 GOGCCFLAGS=-m64: specifies x86, 64-bit GCC.
RUN CGO_ENABLED=1 GOARCH=amd64 GOGCCFLAGS=-m64 GOOS=linux go build -mod=readonly -v -o server

# Stage 1: Run #
FROM alpine:3.16

# Install gcompat to run the binary on Alpine
RUN apk add --no-cache gcompat

# Copy the binary to the production image from the BUILDER stage.
COPY --from=BUILDER /app/server /server

# Run the web service on container startup.
ENTRYPOINT ["/server"]

LABEL org.opencontainers.image.source=https://github.com/vhive-serverless/invitro