FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git
WORKDIR /build
COPY go.mod go.sum ./
RUN GOTOOLCHAIN=auto go mod download
COPY . .
RUN GOTOOLCHAIN=auto CGO_ENABLED=0 go build -o /build/transaction-service ./cmd/api

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /build/transaction-service /transaction-service
COPY internal/infrastructure/postgres/migrations /migrations
EXPOSE 8081
ENTRYPOINT ["/transaction-service"]
