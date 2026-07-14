FROM --platform=$BUILDPLATFORM tonistiigi/xx AS xx

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder

ARG TARGETPLATFORM
ARG BUILDPLATFORM

RUN apk add clang lld bash
COPY --from=xx / /

WORKDIR /updatejson

COPY go.mod go.sum ./
RUN xx-go mod download && xx-go mod verify

COPY . .
RUN xx-apk add musl-dev gcc
RUN xx-go build -buildvcs=false -o /go/bin/updatejson github.com/cfwidget/updatejson && \
    xx-verify /go/bin/updatejson

FROM alpine
COPY --from=builder /go/bin/updatejson /go/bin/updatejson

WORKDIR /updatejson

EXPOSE 8080

ENV DB_ENGINE="sqlite3" \
    DB_USER="" \
    DB_PASS="" \
    DB_HOST="" \
    DB_FILE="/database/updatejson.db" \
    DB_DATABASE="widget" \
    GIN_MODE="release" \
    DB_MODE="release" \
    CORE_KEY="" \
    CACHE_TTL="1h" \
    HOST="curseupdate.com"

VOLUME /database

ENTRYPOINT ["/go/bin/updatejson"]
CMD [""]