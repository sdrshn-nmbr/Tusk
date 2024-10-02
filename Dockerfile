FROM golang:1.22-alpine

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

COPY web/templates ./web/templates

RUN go build -o main ./cmd/server

CMD ["./main"]
