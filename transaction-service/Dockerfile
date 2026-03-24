FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git
WORKDIR /build

# Copy proto module first (needed by replace directive)
COPY proto/ /build/proto/

# Copy transaction-service
COPY transaction-service/go.mod transaction-service/go.sum ./transaction-service/
WORKDIR /build/transaction-service
RUN GOTOOLCHAIN=auto go mod download

COPY transaction-service/ /build/transaction-service/
RUN GOTOOLCHAIN=auto CGO_ENABLED=0 go build -o /build/transaction-service-bin ./cmd/api

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /build/transaction-service-bin /transaction-service
EXPOSE 8081
ENTRYPOINT ["/transaction-service"]
