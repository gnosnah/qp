fmt:
	gofmt -w -l .

lint:
	golangci-lint run ./...

test:
	go test -race -v ./...

fuzz:
	go test -run '^$$' -fuzz Fuzz_Ops -fuzztime 60s .

bench:
	go test -bench .

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
