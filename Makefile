run: 
	go build -o bin/subpub ./subpub.go
	./bin/subpub

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