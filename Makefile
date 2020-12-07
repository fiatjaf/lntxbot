lntxbot: $(shell find . -name "*.go") bindata.go
	go build -ldflags="-s -w" -o ./lntxbot

bindata.go: $(shell find templates) $(shell find static)
	go-bindata -ignore=node_modules static/... templates/...

deploy: lntxbot
	rsync lntxbot hulsmann:.lightning1/plugins/lntxbot-new
	ssh hulsmann 'ln1 plugin stop lntxbot; mv .lightning1/plugins/lntxbot-new .lightning1/plugins/lntxbot; ln1 plugin start $$HOME/.lightning1/plugins/lntxbot'
