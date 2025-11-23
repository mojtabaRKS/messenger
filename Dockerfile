FROM golang:1.25.3-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o messenger ./cmd/main.go

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o simulator ./test/simulator.go

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/messenger .

COPY --from=builder /app/simulator .

COPY --from=builder /app/migrations ./migrations

COPY --from=builder /app/test ./test

EXPOSE 80

CMD ["./messenger", "server"]

