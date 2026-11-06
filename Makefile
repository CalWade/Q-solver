.PHONY: dev check unit integration e2e build bindings
dev:
	node scripts/run.mjs dev
check:
	node scripts/run.mjs check
unit:
	go test -race ./internal/input/... ./pkg/config ./pkg/shortcut
	npm --prefix frontend test
integration:
	go test -race ./internal/harness/...
	node --test scripts/run.test.mjs
e2e:
	npm --prefix frontend run test:e2e
build:
	node scripts/run.mjs build
bindings:
	node scripts/run.mjs bindings
