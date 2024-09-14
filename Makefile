.PHONY: lint
lint:
	@echo "Running lint scripts.";
	@go vet ./...;
	@go fmt ./...;
	@echo "Done running lint scripts.";


.PHONY: test
test: lint
	@echo "Beginning tests.";
	@echo "Warning: This will take > 10 seconds to run."
	@go test -race -vet=off ./...;
	@echo "Go tests passed.";

.PHONY: test-watch
test-watch:
	@reflex --decoration=none -s -- sh -c "$(MAKE) test";