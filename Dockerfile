FROM --platform=$BUILDPLATFORM tonistiigi/xx AS xx

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder

ARG TARGETPLATFORM
ARG BUILDPLATFORM

COPY --from=xx / /

WORKDIR /updatejson

COPY go.mod go.sum ./
RUN xx-go mod download && xx-go mod verify

COPY . .
RUN xx-go build -buildvcs=false -o /go/bin/updatejson github.com/cfwidget/updatejson && \
    xx-verify /go/bin/updatejson

FROM alpine
COPY --from=builder /go/bin/updatejson /go/bin/updatejson

WORKDIR /updatejson

EXPOSE 8080

ENV   DB_USER="updatejson" \
      DB_PASS="updatejson" \
      DB_HOST="database" \
      DB_DATABASE="widget" \
      GIN_MODE="release" \
      DB_MODE="release" \
      CORE_KEY="" \
      CACHE_TTL="1h" \
      HOST="curseupdate.com"

ENTRYPOINT ["/go/bin/updatejson"]
CMD [""]