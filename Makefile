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

.PHONY: test-watch
test-watch:
	@reflex --decoration=none -s -- sh -c "clear && $(MAKE) test";