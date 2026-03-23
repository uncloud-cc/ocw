.PHONY: install build start dev clean

install:
	pnpm install

build:
	pnpm build

start:
	pnpm start

dev:
	pnpm dev

watch:
	cd .. && ocw watch.yaml

clean:
	rm -rf dist
	rm -rf node_modules
	rm -f pnpm-lock.yaml

rebuild: clean install build

.PHONY: help
help:
	@echo "Available commands:"
	@echo "  make install  - Install dependencies"
	@echo "  make build    - Build TypeScript"
	@echo "  make start    - Start the server"
	@echo "  make dev      - Run in development mode"
	@echo "  make watch    - Run with OCW watch mode"
	@echo "  make clean    - Remove build artifacts"
	@echo "  make rebuild  - Clean and rebuild everything"
