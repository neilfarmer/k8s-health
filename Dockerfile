# Distroless final image. Goreleaser builds the binary outside Docker and
# COPYs it in, but this Dockerfile also supports a standalone `docker build`
# via the build stage below for ad-hoc development images.

# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.24

FROM golang:${GO_VERSION}-alpine AS build
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build \
    -trimpath \
    -ldflags "-s -w \
      -X github.com/neilfarmer/k8s-health/internal/version.Version=${VERSION} \
      -X github.com/neilfarmer/k8s-health/internal/version.Commit=${COMMIT} \
      -X github.com/neilfarmer/k8s-health/internal/version.Date=${DATE}" \
    -o /out/khealth ./cmd/khealth

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/khealth /usr/local/bin/khealth
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/khealth"]
