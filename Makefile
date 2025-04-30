fmt:
	gofmt -w -l .

lint:
	golangci-lint run ./...

test:
	go test -v ./...

bench:
	go test -bench .

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
