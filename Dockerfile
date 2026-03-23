FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git
WORKDIR /build
COPY go.mod go.sum ./
RUN GOTOOLCHAIN=auto go mod download
COPY . .
RUN GOTOOLCHAIN=auto CGO_ENABLED=0 go build -o /build/fraud-detection .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /build/fraud-detection /fraud-detection
EXPOSE 8080
ENTRYPOINT ["/fraud-detection"]
