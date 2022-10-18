MKFILE_PATH := $(lastword $(MAKEFILE_LIST))
CURRENT_DIR := $(patsubst %/,%,$(dir $(realpath $(MKFILE_PATH))))

###Build parameters
GO_BUILD := go build -ldflags '-s -w -extldflags "-static"' -a
PLUGIN_DIR := bin
PLUGIN_NAME := vault-plugin-secrets-datastax-astra
PLUGIN_PATH := $(PLUGIN_DIR)/$(PLUGIN_NAME)
VERSION:=v1.0.0
DIST_DIR := bin/dist

build:
		env CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 $(GO_BUILD) -o $(PLUGIN_DIR)/linux/$(PLUGIN_NAME) || exit 1
		env CGO_ENABLED=0 GOOS=linux   GOARCH=386   $(GO_BUILD) -o $(PLUGIN_DIR)/linux86/$(PLUGIN_NAME) || exit 1
		env CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 $(GO_BUILD) -o $(PLUGIN_DIR)/darwin/$(PLUGIN_NAME) || exit 1
		env CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO_BUILD) -o $(PLUGIN_DIR)/windows/$(PLUGIN_NAME).exe || exit 1
		env CGO_ENABLED=0 GOOS=windows GOARCH=386   $(GO_BUILD) -o $(PLUGIN_DIR)/windows86/$(PLUGIN_NAME).exe || exit 1
		chmod +x $(PLUGIN_DIR)/*
		shasum -a 256 "$(PLUGIN_DIR)/linux/$(PLUGIN_NAME)" | cut -d' ' -f1 > $(PLUGIN_DIR)/linux/$(PLUGIN_NAME).SHA256SUM
		shasum -a 256 "$(PLUGIN_DIR)/linux86/$(PLUGIN_NAME)" | cut -d' ' -f1 > $(PLUGIN_DIR)/linux86/$(PLUGIN_NAME).SHA256SUM
		shasum -a 256 "$(PLUGIN_DIR)/darwin/$(PLUGIN_NAME)" | cut -d' ' -f1 > $(PLUGIN_DIR)/darwin/$(PLUGIN_NAME).SHA256SUM
		shasum -a 256 "$(PLUGIN_DIR)/windows/$(PLUGIN_NAME).exe" | cut -d' ' -f1 > $(PLUGIN_DIR)/windows/$(PLUGIN_NAME).exe.SHA256SUM
		shasum -a 256 "$(PLUGIN_DIR)/windows86/$(PLUGIN_NAME).exe" | cut -d' ' -f1 > $(PLUGIN_DIR)/windows86/$(PLUGIN_NAME).exe.SHA256SUM

build_dev:
	go build -o vault/plugins/vault-plugin-secrets-datastax-astra cmd/vault-plugin-secrets-datastax-astra/main.go

compress:
	rm -f $(DIST_DIR)/*
	zip -j "$(CURRENT_DIR)/$(DIST_DIR)/$(PLUGIN_NAME)_$(VERSION)_linux.zip" \
		"$(PLUGIN_DIR)/linux/$(PLUGIN_NAME)" "$(PLUGIN_DIR)/linux/$(PLUGIN_NAME).SHA256SUM" || exit 1
	zip -j "$(CURRENT_DIR)/$(DIST_DIR)/$(PLUGIN_NAME)_$(VERSION)_linux86.zip" \
		"$(PLUGIN_DIR)/linux86/$(PLUGIN_NAME)" "$(PLUGIN_DIR)/linux86/$(PLUGIN_NAME).SHA256SUM" || exit 1
	zip -j "$(CURRENT_DIR)/$(DIST_DIR)/$(PLUGIN_NAME)_$(VERSION)_darwin.zip" \
		"$(PLUGIN_DIR)/darwin/$(PLUGIN_NAME)" "$(PLUGIN_DIR)/darwin/$(PLUGIN_NAME).SHA256SUM" || exit 1
	zip -j "$(CURRENT_DIR)/$(DIST_DIR)/$(PLUGIN_NAME)_$(VERSION)_windows.zip" \
		"$(PLUGIN_DIR)/windows/$(PLUGIN_NAME).exe" "$(PLUGIN_DIR)/windows/$(PLUGIN_NAME).exe.SHA256SUM" || exit 1
	zip -j "$(CURRENT_DIR)/$(DIST_DIR)/$(PLUGIN_NAME)_$(VERSION)_windows86.zip"\
		"$(PLUGIN_DIR)/windows86/$(PLUGIN_NAME).exe" "$(PLUGIN_DIR)/windows86/$(PLUGIN_NAME).exe.SHA256SUM" || exit 1

vault_server:
	vault server -dev -dev-root-token-id=root -dev-plugin-dir=./vault/plugins -log-level=debug

vault_plugin:
	vault secrets enable -path=astra vault-plugin-secrets-datastax-astra
	vault write astra/config org_id="$ORG_UUID" astra_token="$TOKEN" url="https://api.astra.datastax.com" logical_name="org_logical_name"

vault_server_screen:
	screen -d -m vault server -dev -dev-root-token-id=root -dev-plugin-dir=./vault/plugins -log-level=debug ; sleep 5
	
vault_enable:
	vault secrets enable -path=astra vault-plugin-secrets-datastax-astra

vault_astra_token:
	vault write astra/config org_id="$ORG_UUID" astra_token="$TOKEN" url="https://api.astra.datastax.com" logical_name="org_logical_name"

auto_roles:
	sh update_roles.sh

vault_token:
	vault write astra/org/token org_id="$ORG_UUID" role_name="logical_role_name"

ready: build vault_server_screen vault_enable vault_astra_token auto_roles