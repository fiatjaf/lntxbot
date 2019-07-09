bot: $(shell find . -name "*.go") bindata.go
	go build -o ./bot

bindata.go: $(shell find static)
	cd static/poker && make
	go-bindata static/...
