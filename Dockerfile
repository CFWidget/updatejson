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

ENV DB_HOST="" \
    DB_USER="" \
    DB_PASS="" \
    DB_DATABASE=""

ENTRYPOINT ["/go/bin/updatejson"]
CMD [""]