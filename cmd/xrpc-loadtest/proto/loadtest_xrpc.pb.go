// Code generated by protoc-gen-go-xrpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-xrpc v1.1.0
// - protoc             v6.30.0
// source: proto/loadtest.proto

package proto

import (
	context "context"
	xrpc "github.com/xornet-sl/go-xrpc/xrpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	proto "google.golang.org/protobuf/proto"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires go-xrpc v1.0.0 or later.
const _ = xrpc.SupportPackageIsVersion1

// LoadTestClient is the client API for LoadTest service.
type LoadTestClient interface {
	Call(ctx context.Context, in *Message, opts ...xrpc.CallOption) (*Message, error)
	Stream(ctx context.Context, opts ...xrpc.CallOption) (LoadTest_StreamClient, error)
}

type loadTestClient struct {
	cc xrpc.InvokableConnection
}

func NewLoadTestClient(cc xrpc.InvokableConnection) LoadTestClient {
	return &loadTestClient{cc}
}

func (c *loadTestClient) Call(ctx context.Context, in *Message, opts ...xrpc.CallOption) (*Message, error) {
	out := new(Message)
	err := c.cc.Invoke(ctx, "proto.LoadTest", "Call", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *loadTestClient) Stream(ctx context.Context, opts ...xrpc.CallOption) (LoadTest_StreamClient, error) {
	stream, err := c.cc.NewStream(ctx, "proto.LoadTest", "Stream", opts...)
	if err != nil {
		return nil, err
	}
	x := &loadTestStreamClient{stream}
	return x, nil
}

type LoadTest_StreamClient interface {
	Send(*Message) error
	Recv() (*Message, error)
	xrpc.RpcStream
}

type loadTestStreamClient struct {
	xrpc.RpcStream
}

func (x *loadTestStreamClient) Send(m *Message) error {
	return x.RpcStream.SendMsg(m)
}

func (x *loadTestStreamClient) Recv() (*Message, error) {
	m := new(Message)
	if err := x.RpcStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// LoadTestServer is the server API for LoadTest service.
// All implementations must embed UnimplementedLoadTestServer
// for forward compatibility
type LoadTestServer interface {
	Call(context.Context, *Message) (*Message, error)
	Stream(LoadTest_StreamServer) error
	mustEmbedUnimplementedLoadTestServer()
}

// UnimplementedLoadTestServer must be embedded to have forward compatible implementations.
type UnimplementedLoadTestServer struct {
}

func (UnimplementedLoadTestServer) Call(context.Context, *Message) (*Message, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Call not implemented")
}
func (UnimplementedLoadTestServer) Stream(LoadTest_StreamServer) error {
	return status.Errorf(codes.Unimplemented, "method Stream not implemented")
}
func (UnimplementedLoadTestServer) mustEmbedUnimplementedLoadTestServer() {}

func RegisterLoadTestServer(s xrpc.ServiceRegistrar, srv LoadTestServer) {
	s.RegisterService(&LoadTest_ServiceDesc, srv)
}

func _LoadTest_Call_Handler(srv interface{}, ctx context.Context, in proto.Message) (proto.Message, error) {
	return srv.(LoadTestServer).Call(ctx, in.(*Message))
}

func _LoadTest_Stream_Handler(srv interface{}, stream xrpc.RpcStream) error {
	return srv.(LoadTestServer).Stream(&loadTestStreamServer{stream})
}

type LoadTest_StreamServer interface {
	Send(*Message) error
	Recv() (*Message, error)
	xrpc.RpcStream
}

type loadTestStreamServer struct {
	xrpc.RpcStream
}

func (x *loadTestStreamServer) Send(m *Message) error {
	return x.RpcStream.SendMsg(m)
}

func (x *loadTestStreamServer) Recv() (*Message, error) {
	m := new(Message)
	if err := x.RpcStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// LoadTest_ServiceDesc is the xrpc.ServiceDesc for LoadTest service.
// It's only intended for direct use with xrpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var LoadTest_ServiceDesc = xrpc.ServiceDesc{
	ServiceName: "proto.LoadTest",
	HandlerType: (*LoadTestServer)(nil),
	Methods: map[string]xrpc.MethodHandler{
		"Call": _LoadTest_Call_Handler,
	},
	Streams: map[string]xrpc.StreamDesc{
		"Stream": {
			Handler:       _LoadTest_Stream_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
}
