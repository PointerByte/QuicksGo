ARG GO_VERSION=1.25-alpine

FROM otel/autoinstrumentation-go:v0.23.0 AS agent

FROM golang:${GO_VERSION} AS build

USER root

# Update system
RUN apk update && apk upgrade && apk add --no-cache build-base

# Env Go
#ENV GOPROXY=https://proxy.golang.org,direct
#ENV GOSUMDB=sum.golang.org
#ENV CGO_ENABLED=1

WORKDIR /app

# Copiar solo dep files para cache
COPY go.mod go.sum ./
RUN go mod download

# Copiar el resto del código
COPY . .

# Build app
RUN go build -o /app/goapp .

# Run

FROM alpine:3.22.2

USER root

WORKDIR /app

COPY --from=build /app/target ./target
COPY --from=build /app/application.json ./application.json
COPY --from=build /app/goapp ./goapp
COPY --from=agent /otel-go-instrumentation ./otel-go-instrumentation

ENV OTEL_GO_AUTO_TARGET_EXE=./goapp
ENV OTEL_TRACES_EXPORTER=none
ENV OTEL_METRICS_EXPORTER=none
ENV OTEL_LOGS_EXPORTER=none

EXPOSE 8080

ENTRYPOINT ["/bin/sh", "-lc", "./goapp & exec ./otel-go-instrumentation attach"]