.PHONY: proto
proto: 
	protoc --plugin=../protoc-gen-go-xrpc/protoc-gen-go-xrpc \
		--go_out=. --go_opt=paths=source_relative \
		--go-xrpc_out=. --go-xrpc_opt=paths=source_relative \
		proto/loadtest.proto
