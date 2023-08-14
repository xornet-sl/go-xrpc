package xrpc

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"

	iutil "github.com/xornet-sl/go-xrpc/xrpc/internal/util"
	"google.golang.org/grpc/metadata"
	"nhooyr.io/websocket"
)

// RpcClient is a thin client fabric mainly needed to pre-register
// services before calling a Dial function
type RpcClient struct {
	services   *ServiceRegistry
	logContext *LogContext
}

func NewClient() *RpcClient {
	logContext := newLogContext()
	logContext.ConnectionType = ConnType_Client
	return &RpcClient{
		services:   NewServiceRegistry(),
		logContext: logContext,
	}
}

func (this *RpcClient) Dial(ctx context.Context, target string, options ...Option) (*RpcConn, error) {
	this.services.Freeze()

	opts := defaultOptions()
	for _, opt := range options {
		opt.apply(opts)
	}

	httpClient := &http.Client{}
	if opts.tlsConfig != nil {
		httpClient.Transport = &http.Transport{
			TLSClientConfig: opts.tlsConfig,
		}
	}

	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
		if opts.tlsConfig != nil {
			target = "https://" + target
		} else {
			target = "http://" + target
		}
	}

	md, _ := metadata.FromOutgoingContext(ctx)
	c, r, err := websocket.Dial(ctx, target, &websocket.DialOptions{
		HTTPClient: httpClient,
		HTTPHeader: iutil.PackHTTPHeaders(md),
	})
	if r != nil {
		if r.StatusCode == http.StatusUnauthorized {
			return nil, errors.New("RPC Dial failed: Unauthorized")
		}
	}
	if err != nil {
		return nil, err
	}

	var retErrorMu sync.Mutex
	retErrorClosed := false
	retError := make(chan error)

	oldOnConnOpen := opts.onConnOpen
	connHook := func(ctx context.Context, conn *RpcConn) (context.Context, error) {
		retErrorMu.Lock()
		if !retErrorClosed {
			retErrorClosed = true
			close(retError)
		}
		retErrorMu.Unlock()

		if oldOnConnOpen != nil {
			return oldOnConnOpen(ctx, conn)
		}
		return nil, nil
	}
	opts.onConnOpen = connHook
	conn := newRpcConn(ConnType_Client, opts, this, target, c, this.services.Services, this.logContext)

	// serve connection in background
	go func() {
		err := conn.serve(ctx)
		if err != nil {
			conn.logContext.logError(err, "conn.serve error")
		}
		retErrorMu.Lock()
		defer retErrorMu.Unlock()
		if !retErrorClosed {
			retErrorClosed = true
			retError <- err
			close(retError)
		}
	}()

	select {
	case err := <-retError:
		if err != nil {
			return nil, err
		}
	}
	return conn, nil
}

func (this *RpcClient) RegisterService(desc *ServiceDesc, impl interface{}) {
	if err := this.services.Register(desc, impl); err != nil {
		panic(err)
	}
}
