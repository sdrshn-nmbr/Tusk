.PHONY: build run test

build:
	npm run build:css
	go build -o tusk ./cmd/server/

run: build
	./tusk

test:
	@if [ -z "$(DIR)" ]; then \
		echo "Usage: make test DIR=<directory_to_test>"; \
		exit 1; \
	fi
	go test -v ./$(DIR) -count=1

# to keep your CSS up-to-date during development
# npx tailwindcss -i ./web/static/css/main.css -o ./web/static/css/styles.css --watch
