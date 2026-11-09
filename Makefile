.PHONY: prepare dev check unit integration e2e build bindings panel-probe host-probe focus-probe capture-probe stream-capture-probe live-capture-probe keyboard-probe compatibility
prepare:
	node scripts/run.mjs prepare
dev:
	node scripts/run.mjs dev
check:
	node scripts/run.mjs check
unit: prepare
	go test -race ./internal/input/... ./internal/window/... ./internal/capture/... ./internal/solve/... ./pkg/config ./pkg/shortcut ./pkg/solution ./pkg/llm ./pkg/broadcast ./pkg/screen ./pkg/state ./app
	npm --prefix frontend test
	node --test scripts/framework.test.mjs scripts/toolchain.test.mjs scripts/compatibility.test.mjs scripts/focus-evidence.test.mjs scripts/keyboard-evidence.test.mjs
integration: prepare
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
host-probe:
	node scripts/run.mjs host-probe
focus-probe:
	node scripts/run.mjs focus-probe
capture-probe:
	node scripts/run.mjs capture-probe
stream-capture-probe:
	node scripts/run.mjs stream-capture-probe
live-capture-probe:
	node scripts/run.mjs live-capture-probe
keyboard-probe:
	node scripts/run.mjs keyboard-probe
compatibility:
	node scripts/compatibility.mjs
