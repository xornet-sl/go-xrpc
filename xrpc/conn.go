package xrpc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	iutil "github.com/xornet-sl/go-xrpc/xrpc/internal/util"
	"github.com/xornet-sl/go-xrpc/xrpc/internal/xrpcpb"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wspb"
)

type ConnType int

const (
	ConnType_Unknown ConnType = iota
	ConnType_Client
	ConnType_Server
)

type InvokableConnection interface {
	Invoke(ctx context.Context, service, method string, in proto.Message, out proto.Message, opts ...CallOption) error
	Notify(ctx context.Context, service, method string, in proto.Message, opts ...CallOption) error
	NewStream(ctx context.Context, service, method string, opts ...CallOption) (RpcStream, error)
}

type invokeResult struct {
	out *anypb.Any
	err error
}

type ctxCancel struct {
	Cancel    context.CancelCauseFunc
	Cancelled bool
}

type connectionParent interface {
	//
}

type transportCtxKey struct{}

// RpcConn represents the specific rpc connection
// it can be produced either by RpcServer by accepting a client on
// a server side or by the Dial method on a client side
type RpcConn struct {
	opts       *options
	connType   ConnType
	logContext *LogContext

	parent      connectionParent
	wc          *websocket.Conn
	services    map[string]*ServiceDesc
	remoteAddr  string
	remoteHello *xrpcpb.Hello
	incomingMD  metadata.MD
	outgoingMD  metadata.MD

	writeQueue      chan []byte
	served          atomic.Bool
	serveCtx        context.Context
	initializedChan chan struct{}
	active          atomic.Bool

	// Outbound invokes
	invokeId  atomic.Uint64
	invokesMu sync.RWMutex
	invokes   map[uint64]chan invokeResult

	// Inbound context cancel functions
	cancelsMu sync.RWMutex
	cancels   map[uint64]ctxCancel

	// TODO: Flow Control

	// Outbound streams
	clientStreamsMu sync.RWMutex
	clientStreams   map[uint64]*rpcStream

	// Inbound streams
	serverStreamsMu sync.RWMutex
	serverStreams   map[uint64]*rpcStream
}

func newRpcConn(connType ConnType, opts *options, parent connectionParent, remoteAddr string,
	wc *websocket.Conn, services map[string]*ServiceDesc, logContext *LogContext) *RpcConn {
	conn := &RpcConn{
		opts:       opts,
		connType:   connType,
		remoteAddr: remoteAddr,
		parent:     parent,
		wc:         wc,
		services:   services,
		logContext: logContext.Dup(),

		initializedChan: make(chan struct{}),
		invokes:         make(map[uint64]chan invokeResult),
		cancels:         make(map[uint64]ctxCancel),
		writeQueue:      make(chan []byte),

		clientStreams: make(map[uint64]*rpcStream),
		serverStreams: make(map[uint64]*rpcStream),
	}
	conn.logContext.RpcConnection = conn
	conn.logContext.Fields["remote"] = remoteAddr
	conn.logContext.logCallback = opts.onLog
	conn.logContext.debugLogCallback = opts.onDebugLog

	conn.wc.SetReadLimit(opts.callOpts.maxMessageSize)

	return conn
}

func (this *RpcConn) Context() context.Context {
	return this.serveCtx
}

func (this *RpcConn) InitDone() <-chan struct{} {
	return this.initializedChan
}

func (this *RpcConn) getTransportMetadata() metadata.MD {
	if this.connType == ConnType_Client {
		return this.outgoingMD
	}
	return this.incomingMD
}

func (this *RpcConn) populateCtxWithTransport(ctx context.Context) context.Context {
	return context.WithValue(ctx, transportCtxKey{}, this)
}

func GetRpcConnFromContext(ctx context.Context) *RpcConn {
	conn, _ := ctx.Value(transportCtxKey{}).(*RpcConn)
	return conn
}

func GetTransportMDFromContext(ctx context.Context) metadata.MD {
	if conn := GetRpcConnFromContext(ctx); conn != nil {
		return conn.getTransportMetadata()
	}
	return metadata.Pairs()
}

func (this *RpcConn) localHello() *xrpcpb.Hello {
	// TODO:
	return &xrpcpb.Hello{
		FlowControl:   false,
		InitialWindow: this.opts.connectionInboundWindow,
	}
}

func (this *RpcConn) RemoteAddr() net.Addr { return iutil.StrAddr(this.remoteAddr) }

func (this *RpcConn) serve(ctx context.Context) (retError error) {
	if this.served.Load() {
		return fmt.Errorf("connections can not be served twice")
	}
	this.served.Store(true)
	this.invokeId.Store(0)

	this.incomingMD, _ = metadata.FromIncomingContext(ctx)
	this.outgoingMD, _ = metadata.FromOutgoingContext(ctx)
	ctx = this.populateCtxWithTransport(ctx)

	defer close(this.writeQueue)

	var cancel context.CancelCauseFunc
	this.serveCtx, cancel = context.WithCancelCause(ctx)
	defer func() { cancel(retError) }()

	closed := false
	defer func() {
		if closed {
			return
		}
		if retError == nil {
			this.wc.Close(websocket.StatusNormalClosure, "")
		} else {
			this.wc.Close(websocket.StatusAbnormalClosure, retError.Error())
		}
	}()
	defer this.active.Store(false)

	defer this.cleanupStreams()

	this.exchangeHello()

	if newCtx, err := this.onOpen(); err != nil {
		this.wc.Close(websocket.StatusInternalError, "connection can not be opened: "+err.Error())
		return fmt.Errorf("connection can not be opened: %w", err)
	} else if newCtx != nil {
		this.serveCtx = newCtx
	}
	defer func() { this.onClose(retError) }()
	this.active.Store(true)
	close(this.initializedChan) // set up initialized flag

	go func() {
		// Writer
		for {
			select {
			case <-this.serveCtx.Done():
				return
			case bMsg, ok := <-this.writeQueue:
				if !ok { // channel closed
					return
				}

				if err := this.wc.Write(this.serveCtx, websocket.MessageBinary, bMsg); err != nil {
					if !isNormalCloseError(err) {
						this.logContext.logError(err, "conn.serve write error")
					}
					return
				}
			}
		}
	}()

	for {
		// Reader
		if retError = this.readWsMessage(); retError != nil {
			if errors.Is(retError, context.Canceled) {
				this.wc.Close(websocket.StatusGoingAway, retError.Error())
				closed = true
			}
			if isNormalCloseError(retError) {
				retError = nil
			} else {
				// this.logContext.logError(retError, "conn.serve message read error")
			}
			return
		}
	}
}

func (this *RpcConn) exchangeHello() error {
	go func() {
		bMsg := <-this.writeQueue
		if err := this.wc.Write(this.serveCtx, websocket.MessageBinary, bMsg); err != nil {
			if websocket.CloseStatus(err) != websocket.StatusNormalClosure {
				this.logContext.logError(err, "conn.serve write error")
			}
		}
	}()

	this.writeWsMessage(this.serveCtx, &xrpcpb.Packet{
		PacketType: &xrpcpb.Packet_System{
			System: &xrpcpb.System{
				SystemType: &xrpcpb.System_Hello{
					Hello: this.localHello(),
				},
			},
		},
	})

	go func() {
		select {
		case <-this.initializedChan:
		case <-this.serveCtx.Done():
		case <-time.After(defaultHelloTimeout):
			this.wc.Close(websocket.StatusProtocolError, "no hello packet received")
		}
	}()

	return this.readWsMessage()
}

func (this *RpcConn) readWsMessage() error {
	var v xrpcpb.Packet
	if err := wspb.Read(this.serveCtx, this.wc, &v); err != nil {
		// We will terminate connection if any wtf is happening on protocol level.
		return err
	}

	if !this.active.Load() {
		var hello *xrpcpb.Hello = nil
		if sys := v.GetSystem(); sys != nil {
			hello = sys.GetHello()
		}
		if hello == nil {
			this.wc.Close(websocket.StatusProtocolError, "invalid hello packet")
			return errors.New("invalid hello packet")
		}
		this.remoteHello = hello
		return nil
	}

	if streamMsg := v.GetStreamMsg(); streamMsg != nil {
		this.processStreamMsg(streamMsg)
		return nil
	} else if request := v.GetRequest(); request != nil {
		go this.processRequest(request)
		return nil
	} else if response := v.GetResponse(); response != nil {
		go this.processResponse(response)
		return nil
	} else if sys := v.GetSystem(); sys != nil {
		if hello := sys.GetHello(); hello != nil {
			return fmt.Errorf("duplicated hello packet")
		} else if ctxCancel := sys.GetCtxCancel(); ctxCancel != nil {
			id := ctxCancel.GetId()
			this.cancelsMu.RLock()
			defer this.cancelsMu.RUnlock()
			if cancel, ok := this.cancels[id]; ok {
				cancel.Cancelled = true
				cancel.Cancel(errors.New(ctxCancel.GetCause()))
			}
			return nil
		}
	}

	return fmt.Errorf("readWsMessage: unknown message type")
}

func (this *RpcConn) processRequest(req *xrpcpb.Request) {
	getLogContext := func() *LogContext {
		logContext := this.logContext.Dup()
		logContext.Fields["request_id"] = req.GetId()
		logContext.Fields["service"] = req.GetService()
		logContext.Fields["method"] = req.GetMethod()
		logContext.Fields["is_notification"] = req.GetIsNotification()
		return logContext
	}

	var resp *anypb.Any
	errorNum := int32(0)
	errorStr := ""

	md := iutil.UnpackInvokeMetadata(req.GetMetadata())
	reqCtx := metadata.NewIncomingContext(this.serveCtx, md)

	if req.GetService() == "" {
		switch req.GetMethod() {
		case "StreamRequest":
			streamReq := new(xrpcpb.StreamRequest)
			if err := req.GetIn().UnmarshalTo(streamReq); err != nil {
				getLogContext().logError(errors.New("malformed StreamRequest"), "can not process a request")
				return
			}
			svc, ok := this.services[streamReq.GetService()]
			if !ok {
				getLogContext().logError(errors.New("service is not registered"), "can not process a stream request")
				return
			}
			var handler StreamHandler
			if streamDesc, ok := svc.Streams[streamReq.GetMethod()]; !ok {
				getLogContext().logError(errors.New("unknown method"), "can not process a stream request")
				return
			} else {
				handler = streamDesc.Handler
			}
			// TODO: stream-bound context
			stream := func() *rpcStream {
				this.serverStreamsMu.Lock()
				defer this.serverStreamsMu.Unlock()
				newStream := newRpcStream(reqCtx, streamReq.GetId(), this, StreamSide_Server)
				this.serverStreams[newStream.id] = newStream
				return newStream
			}()
			defer func() {
				go this.handleIncomingStream(svc.Impl, handler, stream)
			}()
			streamResp := &xrpcpb.StreamResponse{
				Id:            streamReq.GetId(),
				InitialWindow: this.opts.callOpts.streamInboundWindow,
			}
			resp, _ = anypb.New(streamResp)
		default:
			getLogContext().logError(errors.New("malformed system request"), "can not process a request")
			return
		}
	} else {
		svc, ok := this.services[req.GetService()]
		if !ok {
			getLogContext().logError(errors.New("service is not registered"), "can not process a request")
			return
		}
		handler, ok := svc.Methods[req.GetMethod()]
		if !ok {
			getLogContext().logError(errors.New("unknown method"), "can not process a request")
			return
		}

		var reqCancel context.CancelCauseFunc
		reqCtx, reqCancel = context.WithCancelCause(reqCtx)
		defer reqCancel(nil)
		this.cancelsMu.Lock()
		this.cancels[req.GetId()] = ctxCancel{Cancel: reqCancel}
		this.cancelsMu.Unlock()
		defer func() {
			this.cancelsMu.Lock()
			defer this.cancelsMu.Unlock()
			delete(this.cancels, req.GetId())
		}()

		reqMsg, err := req.GetIn().UnmarshalNew()
		if err != nil {
			getLogContext().logError(errors.New("unknown input value"), "can not process a request")
			return
		}
		out, err := handler(svc.Impl, reqCtx, reqMsg)
		if req.GetIsNotification() {
			return
		}

		// Don't respond on context cancel. The counterpart doesn't wait for it.
		this.cancelsMu.RLock()
		if this.cancels[req.GetId()].Cancelled {
			this.cancelsMu.RUnlock()
			return
		}
		this.cancelsMu.RUnlock()

		if err != nil {
			errorNum = 1
			errorStr = err.Error()
		} else if resp, err = anypb.New(out); err != nil {
			resp = nil
			errorNum = 1
			errorStr = err.Error()
		}
	}

	this.writeWsMessage(this.serveCtx, &xrpcpb.Packet{
		PacketType: &xrpcpb.Packet_Response{
			Response: &xrpcpb.Response{
				Id:       req.GetId(),
				Response: resp,
				ErrorNum: errorNum,
				Error:    errorStr,
			},
		},
	})
}

func (this *RpcConn) processResponse(res *xrpcpb.Response) {
	this.invokesMu.RLock()
	defer this.invokesMu.RUnlock()

	if ir, ok := this.invokes[res.GetId()]; ok {
		var err error = nil
		if res.GetError() != "" {
			err = fmt.Errorf(res.GetError())
		}
		ir <- invokeResult{
			out: res.Response,
			err: err,
		}
	} else {
		this.logContext.log("unknown response received")
	}
}

func (this *RpcConn) handleIncomingStream(srv interface{}, handler StreamHandler, stream *rpcStream) {
	defer func() {
		this.serverStreamsMu.Lock()
		defer this.serverStreamsMu.Unlock()
		delete(this.serverStreams, stream.id)
	}()
	err := handler(srv, stream)
	if errors.Is(err, StreamClosedError) {
		// No op. stream is already closed on remote side.
	} else if err != nil {
		stream.close()
		stream.sendClose(2, err.Error())
	} else {
		stream.Close()
	}
}

func (this *RpcConn) processStreamMsg(msg *xrpcpb.StreamMsg) {
	streamId := msg.GetStreamId()
	sender := msg.GetSender()
	var stream *rpcStream
	if sender == xrpcpb.StreamSender_RPC_SERVER {
		// Remote part is a server. We are the client
		this.clientStreamsMu.RLock()
		stream = this.clientStreams[streamId]
		this.clientStreamsMu.RUnlock()
	} else {
		// Remote part is a client. We are the server
		this.serverStreamsMu.RLock()
		stream = this.serverStreams[streamId]
		this.serverStreamsMu.RUnlock()
	}
	if stream == nil {
		this.logContext.logError(fmt.Errorf("unkown stream %d message received", streamId), "unable to process stream message")
		return
	}
	switch msg.MsgType.(type) {
	case *xrpcpb.StreamMsg_Msg:
		stream.onSocketRead(msg)
	case *xrpcpb.StreamMsg_SetWindow:
		// TODO:
	case *xrpcpb.StreamMsg_Close:
		// Remote side closed the stream
		stream.closeWithRemoteError(msg.GetClose().GetError())
	}
}

func (this *RpcConn) cleanupStreams() {
	func() {
		this.serverStreamsMu.Lock()
		defer this.serverStreamsMu.Unlock()
		for id, stream := range this.serverStreams {
			stream.close()
			delete(this.serverStreams, id)
		}
	}()
	func() {
		this.clientStreamsMu.Lock()
		defer this.clientStreamsMu.Unlock()
		for id, stream := range this.clientStreams {
			stream.close()
			delete(this.clientStreams, id)
		}
	}()
}

func (this *RpcConn) writeWsMessage(ctx context.Context, wsMsg *xrpcpb.Packet) error {
	bMsg, err := proto.Marshal(wsMsg)
	if err != nil {
		return fmt.Errorf("writeMessage: unable to marshal message: %w", err)
	}

	defer func() {
		if err := recover(); err != nil {
			// Socket closed
			this.logContext.log("writeWsMessage: channel closed")
		}
	}()
	select {
	case <-this.serveCtx.Done():
		return this.serveCtx.Err()
	case <-ctx.Done():
		return ctx.Err()
	case this.writeQueue <- bMsg:
		return nil
	}
}

func (this *RpcConn) getNextInvokeId() uint64 {
	return this.invokeId.Add(1)
}

func (this *RpcConn) createRequest(ctx context.Context, service, method string, isNotification bool, in proto.Message) (*xrpcpb.Request, error) {
	inValue, err := anypb.New(in)
	if err != nil {
		return nil, err
	}

	md, _ := metadata.FromOutgoingContext(ctx)

	request := &xrpcpb.Request{
		Id:             this.getNextInvokeId(),
		Service:        service,
		Method:         method,
		In:             inValue,
		IsNotification: isNotification,
		Metadata:       iutil.PackInvokeMetadata(md),
	}
	return request, nil
}

func (this *RpcConn) Invoke(ctx context.Context, service string, method string, in proto.Message, out proto.Message, opts ...CallOption) error {
	callOpts := combineCallOpts(&this.opts.callOpts, opts...)

	<-this.initializedChan

	request, err := this.createRequest(ctx, service, method, false, in)
	if err != nil {
		return err
	}
	getLogContext := func() *LogContext {
		logContext := this.logContext.Dup()
		logContext.Fields["request_id"] = request.GetId()
		logContext.Fields["service"] = service
		logContext.Fields["method"] = method
		return logContext
	}

	waitChan := make(chan invokeResult)
	this.invokesMu.Lock()
	this.invokes[request.Id] = waitChan
	this.invokesMu.Unlock()
	defer func() {
		this.invokesMu.Lock()
		defer this.invokesMu.Unlock()
		delete(this.invokes, request.Id)
		close(waitChan)
	}()

	if err = this.writeWsMessage(this.serveCtx, &xrpcpb.Packet{
		PacketType: &xrpcpb.Packet_Request{
			Request: request,
		},
	}); err != nil {
		return err
	}

	if callOpts.callTimeout != 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, callOpts.callTimeout)
		defer cancel()
	}

	select {
	case <-this.serveCtx.Done():
		return this.serveCtx.Err()
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			getLogContext().logError(ctx.Err(), "RPC call timeout")
		}
		if err = this.writeWsMessage(this.serveCtx, &xrpcpb.Packet{
			PacketType: &xrpcpb.Packet_System{
				System: &xrpcpb.System{
					SystemType: &xrpcpb.System_CtxCancel{
						CtxCancel: &xrpcpb.CtxCancel{
							Id:    request.GetId(),
							Cause: ctx.Err().Error(),
						},
					},
				},
			},
		}); err != nil {
			return errors.Join(err, ctx.Err())
		}
		return ctx.Err()
	case result := <-waitChan:
		if result.err == nil {
			result.err = result.out.UnmarshalTo(out)
		}
		return result.err
	}
}

func (this *RpcConn) Notify(ctx context.Context, service string, method string, in proto.Message, opts ...CallOption) error {
	// callOpts := combineCallOpts(&this.opts.callOpts, opts...)

	<-this.initializedChan

	request, err := this.createRequest(ctx, service, method, true, in)
	if err != nil {
		return err
	}

	if err = this.writeWsMessage(this.serveCtx, &xrpcpb.Packet{
		PacketType: &xrpcpb.Packet_Request{
			Request: request,
		},
	}); err != nil {
		return err
	}
	return nil
}

func (this *RpcConn) NewStream(ctx context.Context, service, method string, opts ...CallOption) (RpcStream, error) {
	callOpts := combineCallOpts(&this.opts.callOpts, opts...)

	<-this.initializedChan

	request := &xrpcpb.StreamRequest{
		Id:            this.getNextInvokeId(),
		Service:       service,
		Method:        method,
		InitialWindow: callOpts.streamInboundWindow,
	}

	md, _ := metadata.FromOutgoingContext(ctx)
	ctx = metadata.NewOutgoingContext(ctx, md)

	response := new(xrpcpb.StreamResponse)
	// TODO: split callTimeout and add streamTimeout
	if err := this.Invoke(ctx, "", "StreamRequest", request, response, opts...); err != nil {
		return nil, err
	}

	stream := func() *rpcStream {
		this.clientStreamsMu.Lock()
		defer this.clientStreamsMu.Unlock()
		newStream := newRpcStream(ctx, request.Id, this, StreamSide_Client)
		this.clientStreams[newStream.id] = newStream
		return newStream
	}()

	return stream, nil
}

func (this *RpcConn) IsActive() bool {
	return this.active.Load()
}

func (this *RpcConn) onOpen() (context.Context, error) {
	if this.opts.onConnOpen != nil {
		return this.opts.onConnOpen(this.serveCtx, this)
	}
	return nil, nil
}

func (this *RpcConn) onClose(closeError error) {
	if !this.active.Load() {
		return
	}
	if this.opts.onConnClosed != nil {
		this.opts.onConnClosed(this.serveCtx, this, closeError)
	}
}

func (this *RpcConn) Close() {
	this.wc.Close(websocket.StatusNormalClosure, "") // TODO:
}

func isNormalCloseError(err error) bool {
	switch {
	case
		err == nil,
		websocket.CloseStatus(err) == websocket.StatusNormalClosure,
		isNetConnClosedErr(err),
		errors.Is(err, context.Canceled):
		return true
	default:
		return false
	}
}

func isNetConnClosedErr(err error) bool {
	switch {
	case
		errors.Is(err, net.ErrClosed),
		errors.Is(err, io.EOF),
		errors.Is(err, syscall.EPIPE):
		return true
	default:
		return false
	}
}
