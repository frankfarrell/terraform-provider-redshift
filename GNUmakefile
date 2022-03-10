TEST?=$$(go list ./... |grep -v 'vendor')
GOFMT_FILES?=$$(find . -name '*.go' |grep -v vendor)

VERSION=v0.0.3
PLATFORM ?= $(shell uname -s | tr '[:upper:]' '[:lower:]')
ARCH ?= $(shell arch)
SHORT_GIT_SHA:=$(shell git rev-parse HEAD | cut -c-7)

NAME=terraform-provider-redshift
BIN_NAME=$(NAME)_$(VERSION)-gh-$(SHORT_GIT_SHA)
FULLNAME=$(NAME)_$(VERSION)
GOPRIVATE=github.com/grnhse


default: build

build: fmtcheck
	go install

test: fmtcheck
	go test ./redshift

testacc: fmtcheck
	TF_ACC=1 go test $(TEST) -v $(TESTARGS) -timeout 120m

vet:
	@echo "go vet ."
	@go vet $$(go list ./... | grep -v vendor/) ; if [ $$? -eq 1 ]; then \
		echo ""; \
		echo "Vet found suspicious constructs. Please check the reported constructs"; \
		echo "and fix them if necessary before submitting the code for review."; \
		exit 1; \
	fi

fmt:
	gofmt -w $(GOFMT_FILES)

fmtcheck:
	@sh -c "'$(CURDIR)/scripts/gofmtcheck.sh'"

errcheck:
	@sh -c "'$(CURDIR)/scripts/errcheck.sh'"

test-compile:
	@if [ "$(TEST)" = "./..." ]; then \
		echo "ERROR: Set TEST to a specific package. For example,"; \
		echo "  make test-compile TEST=./aws"; \
		exit 1; \
	fi
	go test -c $(TEST) $(TESTARGS)

build-dist:
	GOOS=darwin GOARCH=amd64 go build -o "dist/darwin/amd64/$(BIN_NAME)"
	GOOS=linux GOARCH=amd64 go build -o "dist/linux/amd64/$(BIN_NAME)"
	GOOS=darwin GOARCH=arm64 go build -o "dist/darwin/arm64/$(BIN_NAME)"

gh-dist: build-dist
	aws s3 cp "dist/darwin/amd64/$(BIN_NAME)" "s3://grnhse-vpc-assets/terraform-plugins/darwin_amd64/$(BIN_NAME)"
	aws s3 cp "dist/linux/amd64/$(BIN_NAME)" "s3://grnhse-vpc-assets/terraform-plugins/linux_amd64/$(BIN_NAME)"
	aws s3 cp "dist/darwin/arm64/$(BIN_NAME)" "s3://grnhse-vpc-assets/terraform-plugins/darwin_arm64/$(BIN_NAME)"

install-local-plugin: build-dist
	mkdir -p $(HOME)/.terraform.d/plugins/tf-registry.greenhouse.dev/grnhse/redshift/$(VERSION)/$(PLATFORM)_$(ARCH)/
	mkdir -p $(HOME)/.terraform.d/plugin-cache
	cp dist/$(PLATFORM)/$(ARCH)/$(BIN_NAME) $(HOME)/.terraform.d/plugins/tf-registry.greenhouse.dev/grnhse/redshift/$(VERSION)/$(PLATFORM)_$(ARCH)/$(BIN_NAME)

clear-local-plugin:
	rm -rf $(HOME)/.terraform.d/plugins/tf-registry.greenhouse.dev/grnhse/redshift

.PHONY: terraform-provider-publisher
terraform-provider-publisher:
ifeq (, $(shell terraform-provider-publisher -version))
	@go install github.com/grnhse/terraform-provider-publisher@latest
else
	@echo "terraform-provider-publisher already installed"
endif

.PHONY: publish-darwin-amd64
publish-darwin-amd64: build terraform-provider-publisher
	terraform-provider-publisher publish dist/darwin/amd64/$(BIN_NAME)

.PHONY: publish-darwin-arm64
publish-darwin-arm64: build terraform-provider-publisher
	terraform-provider-publisher publish dist/darwin/arm64/$(BIN_NAME)

.PHONY: publish-linux
publish-linux: build terraform-provider-publisher
	terraform-provider-publisher publish dist/linux/amd64/$(BIN_NAME)

.PHONY: publish
publish: publish-darwin-arm64 publish-darwin-amd64 publish-linux
	git tag $(VERSION)
	git push --tags

.PHONY: build test testacc vet fmt fmtcheck errcheck test-compile install-local-plugin clear-local-plugin
