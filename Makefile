.PHONY: dev check unit integration e2e build bindings panel-probe compatibility
dev:
	node scripts/run.mjs dev
check:
	node scripts/run.mjs check
unit:
	go test -race ./internal/input/... ./internal/window/... ./internal/capture/... ./internal/solve/... ./pkg/config ./pkg/shortcut ./pkg/solution ./pkg/llm ./pkg/broadcast ./pkg/screen ./pkg/state ./app
	npm --prefix frontend test
	node --test scripts/toolchain.test.mjs scripts/compatibility.test.mjs
integration:
	go test -race ./internal/harness/...
	node --test scripts/run.test.mjs
e2e:
	npm --prefix frontend run test:e2e
build:
	node scripts/run.mjs build
bindings:
	node scripts/run.mjs bindings
panel-probe:
	node scripts/run.mjs panel-probe
compatibility:
	node scripts/compatibility.mjs
