ARG GO_VERSION=1.25.1-alpine

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

FROM alpine:3.15.11

USER root

WORKDIR /app

COPY --from=build /app/target ./target
COPY --from=build /app/application.json ./application.json
COPY --from=build /app/goapp ./goapp

EXPOSE 8080

ENTRYPOINT ["./goapp"]