PROTO_PATH=api/grpc/subscription/v1
PROTO_FILE=subscription.proto
OUT_DIR=pkg/subscription-service/gen/go

run: 
	go build -o bin/main ./cmd/subscription-service/main.go
	./bin/main

up:
	docker compose up -d

down:
	docker compose down

tidy:
	go mod tidy

lint: tidy
	gofumpt -w .
	gci write . --skip-generated -s standard -s default
	golangci-lint run ./...

test: 
	go test -v -coverpkg=./... -coverprofile=coverage.txt -covermode atomic
	go tool cover -func=coverage.txt | grep 'total'
	which gocover-cobertura || go install github.com/t-yuki/gocover-cobertura@latest
	gocover-cobertura < coverage.txt > coverage.xml

generate_grpc:
	protoc --proto_path=$(PROTO_PATH) \
	--go_out=$(OUT_DIR) --go_opt=paths=source_relative \
	--go-grpc_out=$(OUT_DIR) --go-grpc_opt=paths=source_relative \
	$(PROTO_PATH)/$(PROTO_FILE)