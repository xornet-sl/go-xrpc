package xrpc

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/gammazero/deque"
	"github.com/xornet-sl/go-xrpc/xrpc/internal/xrpcpb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

type StreamHandler func(srv interface{}, stream RpcStream) error

type StreamSide int32

var StreamClosedError = errors.New("Stream is closed")

const (
	StreamSide_Client StreamSide = 0
	StreamSide_Server StreamSide = 1
)

type StreamDesc struct {
	Handler       StreamHandler // the handler called for the method
	ServerStreams bool          // indicates the server can perform streaming sends
	ClientStreams bool          // indicates the client can perform streaming sends
}

type RpcStream interface {
	SendMsg(m proto.Message) error
	RecvMsg(m proto.Message) error
	CloseSend() error
	Close()
	CloseWithError(err error)
}

type rpcStream struct {
	streamCtx context.Context
	id        uint64
	sendSeq   atomic.Uint64
	conn      *RpcConn
	side      StreamSide

	// Flow Control
	fcRemoteEnabled bool
	fcRemoteWindow  WindowSize
	fcLocalEnabled  bool
	fcLocalWindow   WindowSize

	// Receive buffer
	queue       deque.Deque[*xrpcpb.StreamMsg]
	active      atomic.Bool
	remoteError string
	buffByteLen atomic.Uint32
	queueMu     sync.Mutex
	queueCond   *sync.Cond
}

func newRpcStream(ctx context.Context, id uint64, parentConnection *RpcConn, side StreamSide) *rpcStream {
	stream := &rpcStream{
		streamCtx: ctx,
		id:        id,
		conn:      parentConnection,
		side:      side,

		fcLocalEnabled: parentConnection.opts.flowControlEnabled,
		fcLocalWindow:  parentConnection.opts.callOpts.streamInboundWindow,
	}
	stream.queueCond = sync.NewCond(&stream.queueMu)
	stream.active.Store(true)
	stream.queue.SetMinCapacity(5) // 2^5 = 32
	return stream
}

// Non-blocking. Called by RpcConn on read from socket. Fails if there is no buffer space left
func (this *rpcStream) onSocketRead(msg *xrpcpb.StreamMsg) error {
	// TODO: check for localwindow
	this.queueMu.Lock()
	defer this.queueMu.Unlock()
	this.queue.PushBack(msg)
	this.queueCond.Signal()
	return nil
}

func (this *rpcStream) getCloseError() error {
	if this.remoteError == "" {
		return StreamClosedError
	} else {
		return fmt.Errorf("stream %d closed with remote side error: %s", this.id, this.remoteError)
	}
}

// Called by client code from stream handler. Blocks until message is sent over the wire
func (this *rpcStream) SendMsg(m proto.Message) error {
	if !this.active.Load() {
		this.queueMu.Lock()
		defer this.queueMu.Unlock()
		return this.getCloseError()
	}

	marshalled, err := anypb.New(m)
	if err != nil {
		return err
	}
	return this.writeStreamMsg(&xrpcpb.StreamMsg{
		StreamId: this.id,
		SeqId:    this.sendSeq.Add(1),
		Sender:   xrpcpb.StreamSender(this.side),
		MsgType: &xrpcpb.StreamMsg_Msg{
			Msg: marshalled,
		},
	})
}

// Called by client code from stram handler. Blocks there is a message from wire
func (this *rpcStream) RecvMsg(m proto.Message) error {
	this.queueMu.Lock()
	defer this.queueMu.Unlock()
	for this.queue.Len() == 0 {
		if !this.active.Load() {
			return this.getCloseError()
		}
		this.queueCond.Wait()
		// TODO: check more exit conditions
	}
	msg := this.queue.PopFront()
	return msg.GetMsg().UnmarshalTo(m)
}

func (this *rpcStream) CloseSend() error {
	return nil
}

func (this *rpcStream) writeStreamMsg(msg *xrpcpb.StreamMsg) error {
	return this.conn.writeWsMessage(this.conn.serveCtx, &xrpcpb.Packet{
		PacketType: &xrpcpb.Packet_StreamMsg{
			StreamMsg: msg,
		},
	})
}

func (this *rpcStream) close() {
	this.closeWithRemoteError("")
}

func (this *rpcStream) closeWithRemoteError(remoteError string) {
	this.queueMu.Lock()
	defer this.queueMu.Unlock()
	if this.active.Load() {
		this.active.Store(false)
		this.remoteError = remoteError
		this.queueCond.Broadcast()
	}
}

func (this *rpcStream) sendClose(errorNum int32, errorMsg string) {
	this.conn.writeWsMessage(this.conn.serveCtx, &xrpcpb.Packet{
		PacketType: &xrpcpb.Packet_StreamMsg{
			StreamMsg: &xrpcpb.StreamMsg{
				StreamId: this.id,
				SeqId:    this.sendSeq.Add(1),
				Sender:   xrpcpb.StreamSender(this.side),
				MsgType: &xrpcpb.StreamMsg_Close{
					Close: &xrpcpb.StreamClose{
						ErrorNum: errorNum,
						Error:    errorMsg,
					},
				},
			},
		},
	})
}

func (this *rpcStream) Close() {
	this.CloseWithError(nil)
}

func (this *rpcStream) CloseWithError(err error) {
	if !this.active.Load() {
		return
	}
	this.close()
	if err == nil {
		this.sendClose(0, "")
	} else {
		this.sendClose(2, err.Error())
	}
}
