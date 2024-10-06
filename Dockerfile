FROM golang:1.22 as builder

WORKDIR /app

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -v -o server ./

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/server .

# You don't need to copy templates anymore, they're embedded
# COPY --from=builder /app/web/templates ./web/templates

COPY --from=builder /app/web/static ./web/static
COPY --from=builder /app/web/templates ./web/templates

EXPOSE 8080

CMD ["./server"]
