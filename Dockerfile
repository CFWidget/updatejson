FROM golang:alpine AS builder

WORKDIR /updatejson
COPY . .

RUN go build -o /go/bin/updatejson github.com/lordralex/updatejson

FROM alpine
COPY --from=builder /go/bin/updatejson /go/bin/updatejson
COPY --from=builder /updatejson/home.html /updatejson/home.html
COPY --from=builder /updatejson/app.css /updatejson/app.css
COPY --from=builder /updatejson/app.js /updatejson/app.js

WORKDIR /updatejson

EXPOSE 8080

ENV   DB_USER="updatejson" \
      DB_PASS="updatejson" \
      DB_HOST="database" \
      DB_DATABASE="widget" \
      GIN_MODE="release" \
      DB_MODE="release" \
      MEMCACHE_SERVERS="" \
      CORE_KEY="" \
      CACHE_TTL="1h"

ENTRYPOINT ["/go/bin/updatejson"]
CMD [""]