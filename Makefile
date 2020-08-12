lntxbot: $(shell find . -name "*.go") bindata.go
	go build -ldflags="-s -w" -o ./lntxbot

bindata.go: $(shell find templates)
	go-bindata -ignore=node_modules static/... templates/...

deploy: lntxbot
	scp lntxbot taniaaustralis-403:.lightning/plugins/lntxbot-new
	ssh taniaaustralis-403 'lightning/cli/lightning-cli plugin stop lntxbot; mv .lightning/plugins/lntxbot-new .lightning/plugins/lntxbot; lightning/cli/lightning-cli plugin start $$HOME/.lightning/plugins/lntxbot'
