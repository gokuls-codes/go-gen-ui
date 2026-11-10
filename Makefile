.PHONY: css-generate
css-generate:
	npx @tailwindcss/cli -i ./static/css/input.css -o ./static/css/styles.css

.PHONY: templ-generate
templ-generate:
	templ generate

.PHONY: run
run: templ-generate css-generate
	go run cmd/main.go