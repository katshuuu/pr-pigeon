# syntax=docker/dockerfile:1

FROM golang:1.22-alpine AS build
WORKDIR /src

# deps are cached separately from the sources
COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/pigeon ./cmd/pigeon

# distroless: no shell, no package manager, runs as non-root, has CA certs for api.telegram.org
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/pigeon /pigeon
EXPOSE 8443
USER nonroot:nonroot
ENTRYPOINT ["/pigeon"]
