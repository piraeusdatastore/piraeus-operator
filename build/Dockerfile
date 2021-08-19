FROM golang:buster AS builder

COPY . /build
WORKDIR /build
RUN CGO_ENABLED=0 go build -o ./_output/bin/piraeus-operator --ldflags '-extldflags "-static"' -gcflags all=-trimpath=. --asmflags all=-trimpath=. github.com/piraeusdatastore/piraeus-operator/cmd/manager

FROM gcr.io/distroless/static:latest
# install operator binary
COPY --from=builder /build/_output/bin/piraeus-operator /usr/local/bin/piraeus-operator
USER 1001
ENTRYPOINT ["/usr/local/bin/piraeus-operator"]
