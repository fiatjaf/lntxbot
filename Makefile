lntxbot: $(shell find . -name "*.go")
	go build -ldflags="-s -w" -o ./lntxbot

deploy: lntxbot
	rsync lntxbot hulsmann:lntxbot/lntxbot-new
	ssh root@hulsmann 'systemctl stop lntxbot'
	ssh hulsmann 'mv lntxbot/lntxbot-new lntxbot/lntxbot'
	ssh root@hulsmann 'systemctl start lntxbot'
