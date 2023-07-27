PROJECT ?= piraeus-operator
REGISTRY ?= piraeusdatastore
TAG ?= latest
NOCACHE ?= false

help:
	@echo "Useful targets: 'update', 'upload'"

all: update upload

.PHONY: update
update:
	docker build -t $(PROJECT):$(TAG) --no-cache=$(NOCACHE) -f build/Dockerfile .
	docker tag $(PROJECT):$(TAG) $(PROJECT):latest


.PHONY: upload
upload:
	for r in $(REGISTRY); do \
		docker tag $(PROJECT):$(TAG) $$r/$(PROJECT):$(TAG) ; \
		docker tag $(PROJECT):$(TAG) $$r/$(PROJECT):latest ; \
		docker push $$r/$(PROJECT):$(TAG) ; \
		docker push $$r/$(PROJECT):latest ; \
	done

test:
	go test ./...

CONTROLLER_GEN = go run sigs.k8s.io/controller-tools/cmd/controller-gen@v0.8.0

deep-copy:
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

crds:
	$(CONTROLLER_GEN) crd paths="./..." output:crd:artifacts:config=config/crd/bases
	mv ./config/crd/bases/* ./charts/piraeus/crds

helm-values:
	cat ./charts/piraeus/values.yaml \
      | sed 's/docker.io/docker.m.daocloud.io/' \
      | sed 's/quay.io/quay.m.daocloud.io/' \
      | sed 's/registry.k8s.io/k8s.m.daocloud.io/' \
      | sed 's/k8s.gcr.io/k8s-gcr.m.daocloud.io/' \
      | sed 's/gcr.io/gcr.m.daocloud.io/' \
      | sed 's/ghcr.io/gcr.m.daocloud.io/' \
      | sed 's/bionic/centos7/' \
      > ./charts/piraeus/values.cn.yaml

release:
	# check that VERSION is set
	@if [ -z "$(VERSION)" ]; then \
		echo "Usage: make prepare-release VERSION=<version>" >&2 ; \
		exit 1 ; \
	fi
	# check that version has expected format
	# regex taken from https://semver.org/
	@if ! echo -e "$(VERSION)" | grep -qP '^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$$' ; then \
		echo "version format: <maj>.<min>.<patch>" >&2 ; \
		exit 1 ; \
	fi
	# check that version does not exist
	@if git rev-parse "v$(VERSION)" >/dev/null 2>&1 ; then \
		echo "version v$(VERSION) already exists" >&2 ; \
		exit 1 ; \
	fi
	# check that working tree is clean
	@if ! git diff-index --quiet HEAD -- ; then \
		echo "Refusing to create release from dirty repository" >&2 ; \
		exit 1; \
	fi
	# replace changelog header "Unreleased" with version and replace link target
	sed 's/^## \[Unreleased\]/## [v$(VERSION)] - $(shell date +%Y-%m-%d)/' -i CHANGELOG.md
	sed 's#^\[Unreleased\]: \(.*\)HEAD$$#[v$(VERSION)]: \1v$(VERSION)#' -i CHANGELOG.md
	# replace go operator version
	sed 's/var Version = ".*"/var Version = "$(VERSION)"/' -i version/version.go
	# replace chart version+appVersion
	yq ".version = \"$(VERSION)\"" -i charts/piraeus/Chart.yaml
	yq ".appVersion = \"$(VERSION)\"" -i charts/piraeus/Chart.yaml
	# set operator image to tagged version
	yq ".operator.image = \"quay.io/piraeusdatastore/piraeus-operator:v$(VERSION)\"" -i charts/piraeus/values.yaml
	yq ".operator.image = \"quay.m.daocloud.io/piraeusdatastore/piraeus-operator:v$(VERSION)\"" -i charts/piraeus/values.cn.yaml
	# update full yaml deployment
	$(MAKE) deploy/piraeus
	git add --update
	# commit as current release + tag
	git commit -aevm "Release v$(VERSION)" --signoff
	# We don't do git tag v$(VERSION) here, as the commit will change once its merged in github
	# add "Unreleased" section at top + create comparison link against current master
	sed 's/^## \[v$(VERSION)\]/## [Unreleased]\n\n## [v$(VERSION)]/' -i CHANGELOG.md
	echo "[Unreleased]: https://github.com/piraeusdatastore/piraeus-operator/compare/v$(VERSION)...HEAD" >> CHANGELOG.md
	# set operator image back to :latest during development
	yq ".operator.image = \"quay.io/piraeusdatastore/piraeus-operator:latest\"" -i charts/piraeus/values.yaml
	yq ".operator.image = \"quay.m.daocloud.io/piraeusdatastore/piraeus-operator:latest\"" -i charts/piraeus/values.cn.yaml
	# reset full yaml deployment
	$(MAKE) deploy/piraeus
	# commit begin of new dev cycle
	git commit -aevm "Prepare next dev cycle" --signoff

.PHONY: deploy/piraeus
deploy/piraeus:
	rm -rf "$@"
	mkdir -p "$@"
	helm dependency build charts/piraeus
	helm template -n default piraeus-op charts/piraeus --set stork.schedulerTag=v1.18.0 --set operator.controller.masterPassphrase=changemeplease --output-dir deploy >/dev/null
	deploy/create-kustomization.bash

list-images:
	./list_images.sh | sort

pull-images:
	./list_images.sh | xargs -tI % docker pull %

save-images: pull-images
	rm -vf ./piraeus_images.tgz
	docker save $$( ./list_images.sh ) \
	| gzip > ./piraeus_images.tgz
	ls -lh ./piraeus_images.tgz

list-images.cn:
	./list_images.sh ./charts/piraeus/values.cn.yaml | sort

pull-images.cn:
	./list_images.sh ./charts/piraeus/values.cn.yaml | xargs -tI % docker pull %

save-images.cn: pull-images.cn
	rm -vf ./piraeus_images.tgz
	docker save $$( ./list_images.sh ./charts/piraeus/values.cn.yaml ) \
	| gzip > ./piraeus_images.tgz
	ls -lh ./piraeus_images.tgz
