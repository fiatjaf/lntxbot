static/js/bundle.min.js: $(shell find src/ -name "*.js") node_modules
	./node_modules/.bin/browserify src/index.js -dv -g [ envify --NODE_ENV production ] > static/js/bundle.min.js

prod: node_modules
	./node_modules/.bin/browserify src/index.js -g [ envify --NODE_ENV production ] -g uglifyify | ./node_modules/.bin/terser --compress --mangle --output static/js/bundle.min.js

node_modules: package.json
	pnpm install
