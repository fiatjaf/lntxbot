bot: $(shell find . -name "*.go") bindata.go
	go build -o ./bot

bindata.go: $(shell find static)
	go-bindata static/...
