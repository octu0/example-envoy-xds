DEBUG_FLAG = $(if $(DEBUG), -debug)

VERSION_GO = version.go

_NAME      = $(shell grep -o 'AppName string = "[^"]*"' $(VERSION_GO)  | cut -d '"' -f2)
_VERSION   = $(shell grep -oE 'Version string = "[0-9]+\.[0-9]+\.[0-9]+"' $(VERSION_GO) | cut -d '"' -f2)

_ENVOY     = "envoy"
_ENVOY_VER = 1.15.3

.PHONY: build
build:
	docker build --build-arg VERSION=$(_VERSION) -t $(_NAME):$(_VERSION) .
	docker tag $(_NAME):$(_VERSION) $(_NAME):latest

.PHONY: build-envoy
build-envoy:
	docker build -f envoy/Dockerfile --build-arg VERSION=$(_ENVOY_VER) -t $(_ENVOY):$(_ENVOY_VER) envoy/
	docker tag $(_ENVOY):$(_ENVOY_VER) $(_ENVOY):latest

.PHONY: gcrpkg
gcrpkg: build
	docker tag $(_NAME):$(_VERSION) docker.pkg.github.com/octu0/example-envoy-xds/$(_NAME):$(_VERSION)
	docker push docker.pkg.github.com/octu0/example-envoy-xds/$(_NAME):$(_VERSION)

.PHONY: gcrpkg-envoy
gcrpkg-envoy: build-envoy
	docker tag $(_ENVOY):$(_ENVOY_VER) docker.pkg.github.com/octu0/example-envoy-xds/$(_ENVOY):$(_ENVOY_VER)
	docker push docker.pkg.github.com/octu0/example-envoy-xds/$(_ENVOY):$(_ENVOY_VER)
