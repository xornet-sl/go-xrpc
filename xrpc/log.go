package xrpc

import (
	"fmt"
)

type OnLogCallback func(logContext *LogContext, err error, msg string)
type OnDebugLogCallback func(logGetter DebugLogGetter)
type DebugLogGetter func() (logContext *LogContext, msg string)

type LogContext struct {
	RpcConnection  *RpcConn
	ConnectionType ConnType
	Fields         map[string]interface{}

	logCallback      OnLogCallback
	debugLogCallback OnDebugLogCallback
}

func newLogContext() *LogContext {
	return &LogContext{
		Fields: make(map[string]interface{}),
	}
}

func (this *LogContext) Dup() *LogContext {
	ret := new(LogContext)
	*ret = *this
	ret.Fields = make(map[string]interface{}, len(this.Fields))
	for k, v := range this.Fields {
		ret.Fields[k] = v
	}
	return ret
}

func (this *LogContext) log(msg string, args ...interface{}) {
	if this.logCallback != nil {
		this.logCallback(this, nil, fmt.Sprintf(msg, args...))
	}
}

func (this *LogContext) logError(err error, msg string, args ...interface{}) {
	if this.logCallback != nil {
		this.logCallback(this, err, fmt.Sprintf(msg, args...))
	}
}

func (this *LogContext) logDebug(fn func() string) {
	if this.debugLogCallback != nil {
		this.debugLogCallback(func() (*LogContext, string) { return this, fn() })
	}
}
