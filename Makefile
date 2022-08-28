lntxbot: $(shell find . -name "*.go")
	CC=$$(which musl-gcc) go build -ldflags='-s -w -linkmode external -extldflags "-static"' -o ./lntxbot

deploy: lntxbot
	ssh root@turgot 'systemctl stop lntxbot'
	rsync lntxbot turgot:lntxbot/lntxbot-new
	ssh turgot 'mv lntxbot/lntxbot-new lntxbot/lntxbot'
	ssh root@turgot 'systemctl start lntxbot'
