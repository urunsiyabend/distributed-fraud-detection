FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git
WORKDIR /build

# Copy proto module first (needed by replace directive)
COPY proto/ /build/proto/

# Copy fraud-detection
COPY fraud-detection/go.mod fraud-detection/go.sum ./fraud-detection/
WORKDIR /build/fraud-detection
RUN GOTOOLCHAIN=auto go mod download

COPY fraud-detection/ /build/fraud-detection/
RUN GOTOOLCHAIN=auto CGO_ENABLED=0 go build -o /build/fraud-api ./cmd/api
RUN GOTOOLCHAIN=auto CGO_ENABLED=0 go build -o /build/fraud-worker ./cmd/worker

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /build/fraud-api /fraud-api
COPY --from=builder /build/fraud-worker /fraud-worker
EXPOSE 8080 50051
ENTRYPOINT ["/fraud-api"]
