package xrpc

import (
	"context"
	"crypto/tls"
	"net/http"
	"time"
)

type WindowSize = uint64

const (
	// defaultCallTimeout time.Duration = 0
	defaultCallTimeout             time.Duration = time.Second * 5
	defaultStreamInboundWindow     WindowSize    = 16 * 1024 * 1024
	defaultFlowControlEnabled      bool          = false
	defaultConnectionInboundWindow WindowSize    = 100 * 1024 * 1024
	defaultHelloTimeout            time.Duration = time.Second * 3
	defaultMaxMessageSize          int64         = 1024 * 1024
)

type Option interface {
	apply(*options)
}

type optionFunc func(*options)

func (f optionFunc) apply(opts *options) {
	f(opts)
}

type CallOption interface {
	apply(*callOptions)
}

type callOptionFunc func(*callOptions)

func (f callOptionFunc) apply(opts *callOptions) {
	f(opts)
}

// Common options

// OnConnOpenCallback should return nil context or context which is based on ctx
type OnConnOpenCallback func(ctx context.Context, conn *RpcConn) (context.Context, error)
type OnConnClosedCallback func(ctx context.Context, conn *RpcConn, closeError error)
type options struct {
	sopts    serverOptions
	copts    clientOptions
	callOpts callOptions

	flowControlEnabled      bool
	connectionInboundWindow WindowSize

	tlsConfig *tls.Config

	onLog        OnLogCallback
	onDebugLog   OnDebugLogCallback
	onConnOpen   OnConnOpenCallback
	onConnClosed OnConnClosedCallback
}

func defaultOptions() *options {
	return &options{
		sopts:                   defaultServerOpts(),
		copts:                   defaultClientOpts(),
		callOpts:                defaultCallOpts(),
		flowControlEnabled:      defaultFlowControlEnabled,
		connectionInboundWindow: defaultConnectionInboundWindow,
	}
}

func WithCallOptions(callOpts ...CallOption) Option {
	return optionFunc(func(o *options) {
		for _, co := range callOpts {
			co.apply(&o.callOpts)
		}
	})
}

func WithFlowControlEnabled(fcEnabled bool) Option {
	return optionFunc(func(o *options) {
		o.flowControlEnabled = fcEnabled
	})
}

func WithConnectionInboundWindow(window WindowSize) Option {
	return optionFunc(func(o *options) {
		o.connectionInboundWindow = window
	})
}

func WithTLSConfig(config *tls.Config) Option {
	return optionFunc(func(o *options) {
		o.tlsConfig = config
	})
}

func WithOnLogCallback(fn OnLogCallback) Option {
	return optionFunc(func(o *options) {
		o.onLog = fn
	})
}

func WithOnDebugLogCallback(fn OnDebugLogCallback) Option {
	return optionFunc(func(o *options) {
		o.onDebugLog = fn
	})
}

func WithConnOpenCallback(onConnOpen OnConnOpenCallback) Option {
	return optionFunc(func(o *options) {
		o.onConnOpen = onConnOpen
	})
}

func WithConnClosedCallback(onConnClosed OnConnClosedCallback) Option {
	return optionFunc(func(o *options) {
		o.onConnClosed = onConnClosed
	})
}

// Server options

type OnClientAuthCallback func(ctx context.Context, w http.ResponseWriter, r *http.Request) (context.Context, error)
type OnServerShutdownCallback func()
type serverOptions struct {
	onClientAuth     OnClientAuthCallback
	onServerShutdown OnServerShutdownCallback
}

func defaultServerOpts() serverOptions {
	return serverOptions{
		//
	}
}

func WithClientAuthCallback(onClientAuth OnClientAuthCallback) Option {
	return optionFunc(func(o *options) {
		o.sopts.onClientAuth = onClientAuth
	})
}

func WithServerShutdownCallback(onServerShutdown OnServerShutdownCallback) Option {
	return optionFunc(func(o *options) {
		o.sopts.onServerShutdown = onServerShutdown
	})
}

// Client options

type clientOptions struct {
	//
}

func defaultClientOpts() clientOptions {
	return clientOptions{
		//
	}
}

// Call options

type callOptions struct {
	callTimeout         time.Duration
	streamInboundWindow WindowSize
	maxMessageSize      int64
}

func defaultCallOpts() callOptions {
	return callOptions{
		callTimeout:         defaultCallTimeout,
		streamInboundWindow: defaultStreamInboundWindow,
		maxMessageSize:      defaultMaxMessageSize,
	}
}

func WithCallTimeout(callTimeout time.Duration) CallOption {
	return callOptionFunc(func(co *callOptions) {
		co.callTimeout = callTimeout
	})
}

func WithStreamInboundWindow(window WindowSize) CallOption {
	return callOptionFunc(func(co *callOptions) {
		co.streamInboundWindow = window
	})
}

func WithMaxMessageSize(maxMessageSize int64) CallOption {
	return callOptionFunc(func(co *callOptions) {
		co.maxMessageSize = maxMessageSize
	})
}

func combineCallOpts(o1 *callOptions, o2 ...CallOption) *callOptions {
	if len(o2) == 0 {
		return o1
	}
	ret := new(callOptions)
	*ret = *o1
	for _, o := range o2 {
		o.apply(ret)
	}
	return ret
}
