FROM golang:1.21-alpine AS builder

WORKDIR /updatejson

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN go build -buildvcs=false -o /go/bin/updatejson github.com/cfwidget/updatejson

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