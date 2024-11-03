// Code generated by protoc-gen-go-xrpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-xrpc v1.1.0
// - protoc             v5.27.3
// source: proto/api.proto

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

// PingReplyerClient is the client API for PingReplyer service.
type PingReplyerClient interface {
	Ping(ctx context.Context, in *PingMessage, opts ...xrpc.CallOption) (*PingMessage, error)
	StreamPingsBoth(ctx context.Context, opts ...xrpc.CallOption) (PingReplyer_StreamPingsBothClient, error)
	StreamPingsIn(ctx context.Context, opts ...xrpc.CallOption) (PingReplyer_StreamPingsInClient, error)
	StreamPingsOut(ctx context.Context, in *PingMessage, opts ...xrpc.CallOption) (PingReplyer_StreamPingsOutClient, error)
}

type pingReplyerClient struct {
	cc xrpc.InvokableConnection
}

func NewPingReplyerClient(cc xrpc.InvokableConnection) PingReplyerClient {
	return &pingReplyerClient{cc}
}

func (c *pingReplyerClient) Ping(ctx context.Context, in *PingMessage, opts ...xrpc.CallOption) (*PingMessage, error) {
	out := new(PingMessage)
	err := c.cc.Invoke(ctx, "proto.PingReplyer", "Ping", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *pingReplyerClient) StreamPingsBoth(ctx context.Context, opts ...xrpc.CallOption) (PingReplyer_StreamPingsBothClient, error) {
	stream, err := c.cc.NewStream(ctx, "proto.PingReplyer", "StreamPingsBoth", opts...)
	if err != nil {
		return nil, err
	}
	x := &pingReplyerStreamPingsBothClient{stream}
	return x, nil
}

type PingReplyer_StreamPingsBothClient interface {
	Send(*PingMessage) error
	Recv() (*PingMessage, error)
	xrpc.RpcStream
}

type pingReplyerStreamPingsBothClient struct {
	xrpc.RpcStream
}

func (x *pingReplyerStreamPingsBothClient) Send(m *PingMessage) error {
	return x.RpcStream.SendMsg(m)
}

func (x *pingReplyerStreamPingsBothClient) Recv() (*PingMessage, error) {
	m := new(PingMessage)
	if err := x.RpcStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *pingReplyerClient) StreamPingsIn(ctx context.Context, opts ...xrpc.CallOption) (PingReplyer_StreamPingsInClient, error) {
	stream, err := c.cc.NewStream(ctx, "proto.PingReplyer", "StreamPingsIn", opts...)
	if err != nil {
		return nil, err
	}
	x := &pingReplyerStreamPingsInClient{stream}
	return x, nil
}

type PingReplyer_StreamPingsInClient interface {
	Send(*PingMessage) error
	CloseAndRecv() (*PingMessage, error)
	xrpc.RpcStream
}

type pingReplyerStreamPingsInClient struct {
	xrpc.RpcStream
}

func (x *pingReplyerStreamPingsInClient) Send(m *PingMessage) error {
	return x.RpcStream.SendMsg(m)
}

func (x *pingReplyerStreamPingsInClient) CloseAndRecv() (*PingMessage, error) {
	if err := x.RpcStream.CloseSend(); err != nil {
		return nil, err
	}
	m := new(PingMessage)
	if err := x.RpcStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *pingReplyerClient) StreamPingsOut(ctx context.Context, in *PingMessage, opts ...xrpc.CallOption) (PingReplyer_StreamPingsOutClient, error) {
	stream, err := c.cc.NewStream(ctx, "proto.PingReplyer", "StreamPingsOut", opts...)
	if err != nil {
		return nil, err
	}
	x := &pingReplyerStreamPingsOutClient{stream}
	if err := x.RpcStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.RpcStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type PingReplyer_StreamPingsOutClient interface {
	Recv() (*PingMessage, error)
	xrpc.RpcStream
}

type pingReplyerStreamPingsOutClient struct {
	xrpc.RpcStream
}

func (x *pingReplyerStreamPingsOutClient) Recv() (*PingMessage, error) {
	m := new(PingMessage)
	if err := x.RpcStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// PingReplyerServer is the server API for PingReplyer service.
// All implementations must embed UnimplementedPingReplyerServer
// for forward compatibility
type PingReplyerServer interface {
	Ping(context.Context, *PingMessage) (*PingMessage, error)
	StreamPingsBoth(PingReplyer_StreamPingsBothServer) error
	StreamPingsIn(PingReplyer_StreamPingsInServer) error
	StreamPingsOut(*PingMessage, PingReplyer_StreamPingsOutServer) error
	mustEmbedUnimplementedPingReplyerServer()
}

// UnimplementedPingReplyerServer must be embedded to have forward compatible implementations.
type UnimplementedPingReplyerServer struct {
}

func (UnimplementedPingReplyerServer) Ping(context.Context, *PingMessage) (*PingMessage, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Ping not implemented")
}
func (UnimplementedPingReplyerServer) StreamPingsBoth(PingReplyer_StreamPingsBothServer) error {
	return status.Errorf(codes.Unimplemented, "method StreamPingsBoth not implemented")
}
func (UnimplementedPingReplyerServer) StreamPingsIn(PingReplyer_StreamPingsInServer) error {
	return status.Errorf(codes.Unimplemented, "method StreamPingsIn not implemented")
}
func (UnimplementedPingReplyerServer) StreamPingsOut(*PingMessage, PingReplyer_StreamPingsOutServer) error {
	return status.Errorf(codes.Unimplemented, "method StreamPingsOut not implemented")
}
func (UnimplementedPingReplyerServer) mustEmbedUnimplementedPingReplyerServer() {}

func RegisterPingReplyerServer(s xrpc.ServiceRegistrar, srv PingReplyerServer) {
	s.RegisterService(&PingReplyer_ServiceDesc, srv)
}

func _PingReplyer_Ping_Handler(srv interface{}, ctx context.Context, in proto.Message) (proto.Message, error) {
	return srv.(PingReplyerServer).Ping(ctx, in.(*PingMessage))
}

func _PingReplyer_StreamPingsBoth_Handler(srv interface{}, stream xrpc.RpcStream) error {
	return srv.(PingReplyerServer).StreamPingsBoth(&pingReplyerStreamPingsBothServer{stream})
}

type PingReplyer_StreamPingsBothServer interface {
	Send(*PingMessage) error
	Recv() (*PingMessage, error)
	xrpc.RpcStream
}

type pingReplyerStreamPingsBothServer struct {
	xrpc.RpcStream
}

func (x *pingReplyerStreamPingsBothServer) Send(m *PingMessage) error {
	return x.RpcStream.SendMsg(m)
}

func (x *pingReplyerStreamPingsBothServer) Recv() (*PingMessage, error) {
	m := new(PingMessage)
	if err := x.RpcStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _PingReplyer_StreamPingsIn_Handler(srv interface{}, stream xrpc.RpcStream) error {
	return srv.(PingReplyerServer).StreamPingsIn(&pingReplyerStreamPingsInServer{stream})
}

type PingReplyer_StreamPingsInServer interface {
	SendAndClose(*PingMessage) error
	Recv() (*PingMessage, error)
	xrpc.RpcStream
}

type pingReplyerStreamPingsInServer struct {
	xrpc.RpcStream
}

func (x *pingReplyerStreamPingsInServer) SendAndClose(m *PingMessage) error {
	return x.RpcStream.SendMsg(m)
}

func (x *pingReplyerStreamPingsInServer) Recv() (*PingMessage, error) {
	m := new(PingMessage)
	if err := x.RpcStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _PingReplyer_StreamPingsOut_Handler(srv interface{}, stream xrpc.RpcStream) error {
	m := new(PingMessage)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(PingReplyerServer).StreamPingsOut(m, &pingReplyerStreamPingsOutServer{stream})
}

type PingReplyer_StreamPingsOutServer interface {
	Send(*PingMessage) error
	xrpc.RpcStream
}

type pingReplyerStreamPingsOutServer struct {
	xrpc.RpcStream
}

func (x *pingReplyerStreamPingsOutServer) Send(m *PingMessage) error {
	return x.RpcStream.SendMsg(m)
}

// PingReplyer_ServiceDesc is the xrpc.ServiceDesc for PingReplyer service.
// It's only intended for direct use with xrpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var PingReplyer_ServiceDesc = xrpc.ServiceDesc{
	ServiceName: "proto.PingReplyer",
	HandlerType: (*PingReplyerServer)(nil),
	Methods: map[string]xrpc.MethodHandler{
		"Ping": _PingReplyer_Ping_Handler,
	},
	Streams: map[string]xrpc.StreamDesc{
		"StreamPingsBoth": {
			Handler:       _PingReplyer_StreamPingsBoth_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
		"StreamPingsIn": {
			Handler:       _PingReplyer_StreamPingsIn_Handler,
			ClientStreams: true,
		},
		"StreamPingsOut": {
			Handler:       _PingReplyer_StreamPingsOut_Handler,
			ServerStreams: true,
		},
	},
}
