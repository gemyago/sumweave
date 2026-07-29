# Adapted from gemyago/golang-backend-boilerplate@798f0dc9fd753481d0d698d8232ea08df44185b6.
.PHONY: dist test clean install-crane docker/local-image docker/remote-image helm-validate

include build.cfg

comma := ,
dist_platforms := $(patsubst %,dist/%,$(subst $(comma), ,$(platforms)))
build_root := $(CURDIR)
ui_embed_dist_dir := $(abspath ../apps/sumweave/internal/api/http/embeddedui/dist)
skills_dist_dir := $(build_root)/dist/platform-agents/skills
crane_version := $(shell cat .crane-version)
host_os := $(shell uname -s)
host_arch := $(shell uname -m | sed -e 's/x86_64/x86_64/' -e 's/aarch64/arm64/')
crane := $(build_root)/bin/crane

ui:
	$(MAKE) -C ../apps/sumweave-ui build-embed-dist EMBED_OUT_DIR="$(ui_embed_dist_dir)"
	test -f "$(ui_embed_dist_dir)/index.html"

skills:
	rm -rf "$(skills_dist_dir)"
	mkdir -p "$(skills_dist_dir)"
	cp -R ../.platform-agents/skills/. "$(skills_dist_dir)/"

dist/%: ui skills
	$(eval GOOS := $(word 1,$(subst /, ,$*)))
	$(eval GOARCH := $(word 2,$(subst /, ,$*)))
	mkdir -p "$(build_root)/dist/$(GOOS)/$(GOARCH)"
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -C ../apps/sumweave -tags=release -o "$(build_root)/dist/$(GOOS)/$(GOARCH)/sumweave" ./cmd/sumweave

dist: $(dist_platforms)
	@printf '%s\n' "Prepared release artifacts in $(build_root)/dist"

$(crane):
	mkdir -p "$(build_root)/bin" "$(build_root)/tmp"
	curl -fsSL "https://github.com/google/go-containerregistry/releases/download/$(crane_version)/go-containerregistry_$(host_os)_$(host_arch).tar.gz" -o "$(build_root)/tmp/crane.tar.gz"
	tar -xzf "$(build_root)/tmp/crane.tar.gz" -C "$(build_root)/bin" crane

install-crane: $(crane)

docker/.labels:
	mkdir -p docker
	printf '%s\n' "org.opencontainers.image.source=\"https://github.com/gemyago/sumweave\"" > $@
	printf '%s\n' "org.opencontainers.image.revision=\"$${GIT_COMMIT_SHA:-$$(git rev-parse --short HEAD)}\"" >> $@

docker/local-image: dist docker/.labels
	docker buildx build --load --platform linux/$$(go env GOARCH) --build-arg RUNTIME_IMAGE="$(docker_runtime_image)" --tag "$(docker_local_registry)/sumweave:local" -f docker/Dockerfile ..

docker/remote-image: dist docker/.labels
	@test -n "$$GIT_REF" && test -n "$$GIT_COMMIT_SHA"
	@tags="$$(bash scripts/resolve-docker-tags.sh --stable-branches "$(stable_branches)" --git-ref "$$GIT_REF" --commit-sha "$$GIT_COMMIT_SHA")"; \
	args=""; for tag in $$tags; do args="$$args --tag $(image_repository):$$tag"; done; \
	docker buildx build --push --platform "$(platforms)" --build-arg RUNTIME_IMAGE="$(docker_runtime_image)" $$args -f docker/Dockerfile ..

test:
	bash scripts/resolve-docker-tags.sh --self-test
	python3 -m unittest discover -s scripts/tests -p 'test_*.py'

clean:
	rm -rf dist bin tmp docker/.labels docker/.local-* docker/.remote-*
