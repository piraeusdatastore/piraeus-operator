# syntax=docker/dockerfile:1.4
# Build the manager binary
FROM --platform=$BUILDPLATFORM golang:1.18 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY cmd/ cmd/
COPY api/ api/
COPY internal/ internal/
COPY pkg/ pkg/

# Build
ARG VERSION=devel
ARG TARGETARCH
ARG TARGETOS
RUN --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -a -ldflags "-X github.com/piraeusdatastore/piraeus-operator/v2/pkg/vars.Version=$VERSION" -o manager ./cmd
RUN --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -a -ldflags "-X github.com/piraeusdatastore/piraeus-operator/v2/pkg/vars.Version=$VERSION" -o gencert ./cmd/gencert

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager /workspace/gencert /
USER 65532:65532

ENTRYPOINT ["/manager"]
