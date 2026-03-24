module github.com/urunsiyabend/distributed-fraud-detection/transaction-service

go 1.25.0

require (
	github.com/go-chi/chi/v5 v5.2.5
	github.com/google/uuid v1.6.0
	github.com/lib/pq v1.12.0
	github.com/sony/gobreaker/v2 v2.4.0
	github.com/urunsiyabend/distributed-fraud-detection/proto v0.0.0
	google.golang.org/grpc v1.79.2
)

require (
	golang.org/x/net v0.51.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260209200024-4cfbd4190f57 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

replace github.com/urunsiyabend/distributed-fraud-detection/proto => ../proto
