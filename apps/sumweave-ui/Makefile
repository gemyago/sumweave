OPENAPI_SPEC := ../../runtime/internal/agentapi/openapi.yaml
OPENAPI_OUT := src/lib/agentapi/agentapi.generated.ts
OPENAPI_TS := $(CURDIR)/node_modules/.bin/openapi-typescript

.PHONY: lint test generate-api check-api build-embed-dist validate-embed-dist

lint:
	npm run lint
	$(MAKE) check-api

test:
	mkdir -p coverage/.tmp
	npm run test:run

generate-api:
	"$(OPENAPI_TS)" "$(OPENAPI_SPEC)" -o "$(OPENAPI_OUT)"

check-api:
	"$(OPENAPI_TS)" "$(OPENAPI_SPEC)" -o "$(OPENAPI_OUT)" --check

build-embed-dist:
	test -n "$(EMBED_OUT_DIR)"
	npm run build -- --outDir "$(EMBED_OUT_DIR)" --emptyOutDir
	$(MAKE) validate-embed-dist EMBED_OUT_DIR="$(EMBED_OUT_DIR)"

validate-embed-dist:
	test -n "$(EMBED_OUT_DIR)"
	test -f "$(EMBED_OUT_DIR)/index.html"
