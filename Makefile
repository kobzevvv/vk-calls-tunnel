.PHONY: build build-linux client server clean

# Build for current platform
build: client server

client:
	go build -o vk-tunnel-client ./cmd/client/

server:
	go build -o vk-tunnel-server ./cmd/server/

# Cross-compile for Linux (for VPS deploy)
build-linux:
	GOOS=linux GOARCH=amd64 go build -o vk-tunnel-server-linux ./cmd/server/
	GOOS=linux GOARCH=amd64 go build -o vk-tunnel-client-linux ./cmd/client/

clean:
	rm -f vk-tunnel-client vk-tunnel-server vk-tunnel-*-linux client server
