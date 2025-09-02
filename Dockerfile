# syntax=docker/dockerfile:1

# Build the application from source
FROM golang:1.25-bookworm AS build-stage

WORKDIR /

ENV DEBIAN_FRONTEND=noninteractive

RUN apt -y update
RUN apt -y upgrade
RUN apt install -y ffmpeg

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /goapp

ENTRYPOINT ["/goapp"]