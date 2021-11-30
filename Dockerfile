FROM golang:alpine AS builder

RUN go build -o /go/bin/updatejson github.com/lordralex/updatejson

FROM alpine
COPY --from=builder /go/bin/updatejson /go/bin/updatejson

EXPOSE 8080

ENV DB_HOST="" \
    DB_USER="" \
    DB_PASS="" \
    DB_DATABASE=""

ENTRYPOINT ["/go/bin/updatejson"]
CMD [""]