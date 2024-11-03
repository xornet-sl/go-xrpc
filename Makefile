all:
	protoc --go_out=. \
		--go_opt=paths=source_relative \
		xrpc/internal/xrpcpb/xrpc.proto

protoc:
	cd cmd/protoc-gen-go-xrpc && go build

testpb: 
	protoc --plugin=cmd/protoc-gen-go-xrpc/protoc-gen-go-xrpc \
		--go_out=. --go_opt=paths=source_relative \
		--go-xrpc_out=. --go-xrpc_opt=paths=source_relative \
		--go-grpc_out=proto/grpc --go-grpc_opt=module=go-xrpc/proto \
		proto/api.proto

cloc:
	@cloc --include-lang=Go --skip-uniqueness cmd internal
