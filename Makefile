bot: $(shell find . -name "*.go") bindata.go
	go build -ldflags="-s -w" -o ./bot

bindata.go: poker $(shell find templates)
	go-bindata -ignore=node_modules static/... templates/...

poker: $(shell find static/poker/src -name "*.js")
	cd static/poker && make

deploy: bot
	ssh root@taniaaustralis-403 'systemctl stop lightningd'
	scp bot taniaaustralis-403:lightning/plugins/bot
	ssh root@taniaaustralis-403 'systemctl start lightningd'
