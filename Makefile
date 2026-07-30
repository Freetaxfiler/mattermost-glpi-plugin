PLUGIN_ID := mattermost-glpi-plugin
PLUGIN_VERSION := 0.2.0

all: build

dist: build
	@mkdir -p dist/$(PLUGIN_ID)/server/dist
	@mkdir -p dist/$(PLUGIN_ID)/webapp/dist
	cp plugin.json dist/$(PLUGIN_ID)/
	cp server/dist/plugin-linux-amd64 dist/$(PLUGIN_ID)/server/dist/
	cp webapp/dist/main.js dist/$(PLUGIN_ID)/webapp/dist/
	cd dist && tar -czf $(PLUGIN_ID)-$(PLUGIN_VERSION).tar.gz $(PLUGIN_ID)/
	rm -rf dist/$(PLUGIN_ID)

build: server-build webapp-build

server-build:
	@mkdir -p server/dist
	@go build -ldflags "-X github.com/Freetaxfiler/mattermost-glpi-plugin/server/commands.PluginVersion=$(PLUGIN_VERSION)" -o server/dist/plugin-linux-amd64 ./server

webapp-build:
	@cd webapp && npm install --silent && npm run build

test:
	@go test ./...

vet:
	@go vet ./...

clean:
	@rm -rf dist server/dist
	@rm -rf webapp/node_modules webapp/dist

.PHONY: all dist build server-build webapp-build test vet clean
