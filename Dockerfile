FROM golang:1.19-alpine AS builder

RUN apk add --no-cache git

WORKDIR /tmp/LazyDocker

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN go build -o ./out/LazyDocker .

FROM alpine:3.17

COPY --from=builder /tmp/LazyDocker/out/LazyDocker /app/LazyDocker
COPY ./pages /app/pages

EXPOSE 8080

CMD ["/app/LazyDocker"]
