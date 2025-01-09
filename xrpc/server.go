package xrpc

import (
	"context"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	iutil "github.com/xornet-sl/go-xrpc/xrpc/internal/util"
	"golang.org/x/time/rate"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

const (
	readTimeout     = time.Second * 10
	writeTimeout    = time.Second * 10
	shutdownTimeout = time.Second * 3
)

var (
	rateLimit = struct {
		Every rate.Limit
		Burst int
	}{
		Every: rate.Every(time.Millisecond * 100),
		Burst: 32,
	}
)

// RcpServer represents the main Rpc server handler structure
type RpcServer struct {
	//
	rateLimiter *rate.Limiter
	//
	opts       *options
	logContext *LogContext

	services *ServiceRegistry

	httpServer atomic.Value
	connMu     sync.Mutex
	conns      map[*RpcConn]struct{}
}

func NewServer(options ...Option) (*RpcServer, error) {
	srv := &RpcServer{
		opts:       defaultOptions(),
		services:   NewServiceRegistry(),
		logContext: newLogContext(),
		conns:      make(map[*RpcConn]struct{}),
	}
	srv.logContext.ConnectionType = ConnType_Server
	for _, opt := range options {
		opt.apply(srv.opts)
	}
	srv.logContext.logCallback = srv.opts.onLog
	srv.logContext.debugLogCallback = srv.opts.onDebugLog
	return srv, nil
}

func (this *RpcServer) RegisterService(desc *ServiceDesc, impl interface{}) {
	if err := this.services.Register(desc, impl); err != nil {
		panic(err)
	}
}

func (this *RpcServer) Serve(ctx context.Context, addr string) error {
	this.services.Freeze()

	ws := wsServer{
		rs: this,
	}

	baseCtx := func(net.Listener) context.Context {
		return ctx
	}
	hs := &http.Server{
		Addr:         addr,
		Handler:      ws,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		TLSConfig:    this.opts.tlsConfig,
		BaseContext:  baseCtx,
	}
	this.httpServer.Store(hs)
	hs.RegisterOnShutdown(this.onShutdown)

	errc := make(chan error, 1)
	go func() {
		if this.opts.tlsConfig != nil {
			errc <- hs.ListenAndServeTLS("", "")
		} else {
			errc <- hs.ListenAndServe()
		}
	}()

	defer func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		hs.Shutdown(shutCtx)
	}()

	var err error
	select {
	case err = <-errc:
		if err == http.ErrServerClosed {
			err = nil
		}
	case <-ctx.Done():
		err = nil
	}

	return err
}

func (this *RpcServer) closeAllConnections() {
	this.connMu.Lock()
	defer this.connMu.Unlock()
	for conn := range this.conns {
		conn.Close()
		delete(this.conns, conn)
	}
}

func (this *RpcServer) Shutdown() {
	hs := this.httpServer.Load().(*http.Server)
	if hs != nil {
		shutCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		hs.Shutdown(shutCtx)
	}
}

func (this *RpcServer) authTransport(ctx context.Context, w http.ResponseWriter, r *http.Request) (context.Context, error) {
	if this.opts.sopts.onClientAuth != nil {
		return this.opts.sopts.onClientAuth(ctx, w, r)
	}
	return ctx, nil
}

func (this *RpcServer) onShutdown() {
	this.closeAllConnections()
	if this.opts.sopts.onServerShutdown != nil {
		this.opts.sopts.onServerShutdown()
	}
}

type wsServer struct {
	rs *RpcServer
}

func (this wsServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logContext := this.rs.logContext.Dup()
	logContext.Fields["remote"] = r.RemoteAddr

	if this.rs.rateLimiter != nil && !this.rs.rateLimiter.Allow() {
		http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
		return
	}

	md, err := iutil.UnpackHTTPHeaders(r.Header)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	pr := &peer.Peer{
		Addr: net.Addr(iutil.StrAddr(r.RemoteAddr)), // TODO: support X-Real-IP / X-Forwarded-For
	}
	// TODO: attach timeouts to context
	ctx := metadata.NewIncomingContext(r.Context(), md)
	ctx = peer.NewContext(ctx, pr)

	if newCtx, err := this.rs.authTransport(ctx, w, r); err != nil {
		// Error body should be set in auth callback
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	} else if newCtx != nil {
		ctx = newCtx
	}

	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		logContext.logError(err, "can not accept a websocket connection")
		return
	}
	defer c.Close(websocket.StatusInternalError, "the sky is falling") // TODO:

	opts := new(options)
	*opts = *this.rs.opts
	conn := newRpcConn(ConnType_Server, opts, this.rs, r.RemoteAddr, c, this.rs.services.Services, logContext)

	func() {
		this.rs.connMu.Lock()
		defer this.rs.connMu.Unlock()
		this.rs.conns[conn] = struct{}{}
	}()
	defer func() {
		this.rs.connMu.Lock()
		defer this.rs.connMu.Unlock()
		delete(this.rs.conns, conn)
	}()

	if err = conn.serve(ctx); err != nil {
		c.Close(websocket.StatusInternalError, err.Error())
	}
}
