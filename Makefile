.PHONY: build dev docker test clean install desktop-dev desktop-build vola-bin

UNAME_S := $(shell uname -s)

# 公共编译 Go 守护进程的目标（支持 macOS Universal 二进制）
vola-bin:
ifeq ($(UNAME_S),Darwin)
	@echo "Detected macOS, building Universal Binary (x86_64 + arm64) for vola daemon..."
	@mkdir -p bin
	GOOS=darwin GOARCH=amd64 go build -o bin/vola-amd64 ./cmd/vola
	GOOS=darwin GOARCH=arm64 go build -o bin/vola-arm64 ./cmd/vola
	rm -f bin/vola
	lipo -create -output bin/vola bin/vola-amd64 bin/vola-arm64
	rm bin/vola-amd64 bin/vola-arm64
else
	@echo "Detected non-macOS system, building standard binary for vola daemon..."
	@mkdir -p bin
	go build -o bin/vola ./cmd/vola
endif

# Build frontend and backend into a single binary
build:
	cd web && npm ci && npm run build
	rm -rf internal/web/dist
	cp -r web/dist internal/web/dist
	$(MAKE) vola-bin
	rm -f bin/vol bin/neu bin/neudrive
	go build -o bin/vol ./cmd/vol
	go build -o bin/neu ./cmd/neu
	go build -o bin/neudrive ./cmd/neudrive

install:
	./tools/install-vola.sh

# Run backend + frontend dev servers (frontend proxies API to backend)
dev:
	@echo "Starting backend on :8080 and frontend dev server on :3000"
	@echo "Use Ctrl-C to stop both."
	@trap 'kill 0' EXIT; \
		VOLA_DEV=1 go run ./cmd/vola server --listen :8080 & \
		cd web && npm run dev & \
		wait

# Build the Docker image with embedded frontend
docker:
	docker build -t vola:latest .

# Run all tests
test:
	go test ./...
	cd web && npm run test

clean:
	rm -rf bin/ internal/web/dist web/dist src-tauri/target/

desktop-dev: vola-bin
	# 确保最新的二进制被拷贝到 src-tauri/bin
	mkdir -p src-tauri/bin
	cp bin/vola src-tauri/bin/vola
	# 启动 Tauri 开发环境
	./web/node_modules/.bin/tauri dev

desktop-build: vola-bin
	# 编译前端并同步 dist
	cd web && npm ci && npm run build
	rm -rf internal/web/dist
	cp -r web/dist internal/web/dist
	# 创建打包资源目录并拷贝二进制文件
	mkdir -p src-tauri/bin
	cp bin/vola src-tauri/bin/vola
	# 编译打包桌面应用
	./web/node_modules/.bin/tauri build
