bot: $(shell find . -name "*.go") bindata.go
	go build -o ./bot

bindata.go: poker
	go-bindata -ignore=node_modules static/...

poker: $(shell find static/poker/src -name "*.js")
	cd static/poker && make
