static/js/bundle.min.js: $(shell find src/ -name "*.js")
	./node_modules/.bin/browserify src/index.js -g [ envify --NODE_ENV production ] -g uglifyify | ./node_modules/.bin/terser --compress --mangle --output static/js/bundle.min.js
