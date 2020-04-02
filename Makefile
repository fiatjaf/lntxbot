lntxbot: $(shell find . -name "*.go") bindata.go
	go build -ldflags="-s -w" -o ./lntxbot

bindata.go: $(shell find templates)
	go-bindata -ignore=node_modules static/... templates/...

deploy: lntxbot
	ssh root@taniaaustralis-403 'systemctl stop lightningd'
	scp lntxbot taniaaustralis-403:lightning/plugins/lntxbot
	ssh root@taniaaustralis-403 'systemctl start lightningd'
