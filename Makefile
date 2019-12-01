bot: $(shell find . -name "*.go") bindata.go
	gox -osarch="freebsd/amd64" -output ./bot

bindata.go: poker $(shell find templates)
	go-bindata -ignore=node_modules static/... templates/...

poker: $(shell find static/poker/src -name "*.js")
	cd static/poker && make
