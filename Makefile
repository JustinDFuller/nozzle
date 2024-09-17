.PHONY: lint
lint:
	@echo "Running lint scripts.";
	@go vet ./...;
	@go fmt ./...;
	@golangci-lint run --fix=true ./...;
	@echo "Done running lint scripts.";

.PHONY: lint-watch
lint-watch:
	@reflex --decoration=none -s -- sh -c "clear && $(MAKE) lint";

.PHONY: test
test: lint
	@echo "Beginning tests.";
	@go test -v -race -vet=off ./...;
	@echo "Go tests passed.";

.PHONY: test-example
test-example:
	@go test -run=Example ./...;

.PHONY: test-watch
test-watch:
	@reflex --decoration=none -s -- sh -c "clear && $(MAKE) test";

.PHONY: bench
bench:
	@go test -run=^$ -v -race -benchmem -bench .;

.PHONY: bench-memprofile
bench-memprofile:
	@go test -run=^$ -bench=. -benchmem -memprofile mem.out;

.PHONY: analyze-memprofile
analyze-memprofile:
	@go tool pprof mem.out;

.PHONY: analyze-build
analyze-build:
	@go build -gcflags="-m" .;