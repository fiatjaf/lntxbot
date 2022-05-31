lntxbot: $(shell find . -name "*.go")
	CC=$$(which musl-gcc) go build -ldflags='-s -w -linkmode external -extldflags "-static"' -o ./lntxbot

deploy: lntxbot
	rsync lntxbot turgot:lntxbot/lntxbot-new
	ssh root@turgot 'systemctl stop lntxbot'
	ssh turgot 'mv lntxbot/lntxbot-new lntxbot/lntxbot'
	ssh root@turgot 'systemctl start lntxbot'
