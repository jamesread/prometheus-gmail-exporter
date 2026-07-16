FROM golang:1.23 AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /prometheus-gmail-exporter ./cmd/prometheus-gmail-exporter/

FROM fedora:40

RUN dnf -y update && dnf -y install ca-certificates && dnf clean all

COPY --from=build /prometheus-gmail-exporter /usr/local/sbin/prometheus-gmail-exporter

ARG VERSION=unknown
ARG GITHUB_SHA=unknown
ENV GITHUB_SHA=$GITHUB_SHA
RUN mkdir /app
RUN echo "${VERSION}:${GITHUB_SHA}" > /app/VERSION
WORKDIR /app

ENTRYPOINT [ "/usr/local/sbin/prometheus-gmail-exporter", "-d" ]

EXPOSE 8080
