lntxbot: $(shell find . -name "*.go")
	go build -ldflags="-s -w" -o ./lntxbot

deploy: lntxbot
	rsync lntxbot turgot:lntxbot/lntxbot-new
	ssh root@turgot 'systemctl stop lntxbot'
	ssh turgot 'mv lntxbot/lntxbot-new lntxbot/lntxbot'
	ssh root@turgot 'systemctl start lntxbot'
