# Adapted from gemyago/golang-backend-boilerplate@798f0dc9fd753481d0d698d8232ea08df44185b6.
os := $(shell uname -s | tr '[:upper:]' '[:lower:]')
arch := $(shell uname -m | sed -e 's/x86_64/amd64/' -e 's/arm64/arm64/' -e 's/aarch64/arm64/')
helm_version := $(shell cat .helm-version)
helm := $(CURDIR)/bin/helm

.PHONY: tools lint render-production clean

$(helm):
	mkdir -p bin tmp
	curl -fsSL "https://get.helm.sh/helm-v$(helm_version)-$(os)-$(arch).tar.gz" -o tmp/helm.tar.gz
	tar -xzf tmp/helm.tar.gz -C tmp
	cp tmp/$(os)-$(arch)/helm $@

tools: $(helm)

lint: $(helm)
	$(helm) lint helm/signal-foundry
	$(helm) template signal-foundry helm/signal-foundry >/dev/null

render-production: $(helm)
	$(helm) template signal-foundry helm/signal-foundry -f helm/signal-foundry/values-production.example.yaml

clean:
	rm -rf bin tmp
