CURRENT_OS=$(shell uname | tr A-Z a-z)
VERSION ?= latest
SED_FLAGS=

ifeq ($(CURRENT_OS),darwin)
	SED_FLAGS=-i.bak
else
	SED_FLAGS=-i
endif

build: build-rsync-server build-taokan-operator

build-rsync-server:
	make -C rsync-server build VERSION=$(VERSION)

build-taokan-operator:
	make -C TaoKan build-image VERSION=$(VERSION)

build-image-tarball:
	@echo "[Build] Image tarball: taokan-$(VERSION).tgz"
	docker save infuseai/taokan:$(VERSION) infuseai/rsync-server:$(VERSION) | gzip -c > taokan-$(VERSION).tgz
	@echo "[Build] Image list:    taokan-$(VERSION).txt"
	@echo "infuseai/taokan:$(VERSION)" > taokan-$(VERSION).txt
	@echo "infuseai/rsync-server:$(VERSION)" >> taokan-$(VERSION).txt
	@echo "[Done]"

deploy: deploy-rsync-server deploy-taokan-operator

deploy-rsync-server:
	make -C rsync-server deploy VERSION=$(VERSION)

deploy-taokan-operator:
	make -C TaoKan deploy-image VERSION=$(VERSION)

package-helm-chart: build-taokan-operator
	@mkdir -p ./build
	@cp -rf ./deployments/helm/TaoKanOperator/ ./build/TaoKanOperator
	@sed $(SED_FLAGS) 's/latest/$(VERSION)/g' ./build/TaoKanOperator/Chart.yaml
	@docker create --name taokan-operator-static-build infuseai/taokan:$(VERSION)
	@docker cp taokan-operator-static-build:/go/src/app/TaoKan/bin/taokan-static-linux ./build/TaoKanOperator/taokan-static-linux
	@tar czf TaoKanOperator-$(VERSION).tar.gz -C ./build ./TaoKanOperator/
	@rm -rf ./build
	@docker rm taokan-operator-static-build
	@echo "[Release] helm cahrt"
	@ls -l TaoKanOperator-$(VERSION).tar.gz


