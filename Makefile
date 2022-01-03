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

deploy: deploy-rsync-server deploy-taokan-operator

deploy-rsync-server:
	make -C rsync-server deploy VERSION=$(VERSION)

deploy-taokan-operator:
	make -C TaoKan deploy-image VERSION=$(VERSION)

package-helm-chart:
	@mkdir -p ./build
	@cp -rf ./deployments/helm/TaoKanOperator/ ./build/TaoKanOperator
	@sed $(SED_FLAGS) 's/latest/$(VERSION)/g' ./build/TaoKanOperator/Chart.yaml
	@tar czf TaoKanOperator-$(VERSION).tar.gz -C ./build ./TaoKanOperator/
	@rm -rf ./build
	@echo "[Release] helm cahrt"
	@ls -l TaoKanOperator-$(VERSION).tar.gz