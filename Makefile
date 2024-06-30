.PHONY: build

build:
    npm run build:css
    go build -o tusk ./cmd/server/

run: build
    ./tusk

# to keep your CSS up-to-date during development
# npx tailwindcss -i ./web/static/css/main.css -o ./web/static/css/styles.css --watch