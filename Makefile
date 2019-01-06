bot: $(shell find . -name "*.go")
	go build -o ./bot

watch:
	find . -name "*.go" | entr -r bash -c 'make bot && ./bot'
