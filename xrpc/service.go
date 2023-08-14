package xrpc

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"

	"google.golang.org/protobuf/proto"
)

type ServiceRegistrar interface {
	RegisterService(desc *ServiceDesc, impl interface{})
}

type MethodHandler func(srv interface{}, ctx context.Context, in proto.Message) (proto.Message, error)

type ServiceDesc struct {
	ServiceName string
	HandlerType interface{}
	Impl        interface{}
	Methods     map[string]MethodHandler
	Streams     map[string]StreamDesc
}

type ServiceRegistry struct {
	mu       sync.Mutex
	frozen   atomic.Bool
	Services map[string]*ServiceDesc
}

func NewServiceRegistry() *ServiceRegistry {
	return &ServiceRegistry{
		Services: map[string]*ServiceDesc{},
	}
}

func (this *ServiceRegistry) Freeze() { this.frozen.Store(true) }

func (this *ServiceRegistry) Register(sd *ServiceDesc, ss interface{}) error {
	if this.frozen.Load() {
		return fmt.Errorf("xrpc: Service can not be registered when Registry is frozen: %v", sd.ServiceName)
	}
	if err := checkServiceImplType(sd, ss); err != nil {
		return err
	}
	this.mu.Lock()
	defer this.mu.Unlock()
	if _, ok := this.Services[sd.ServiceName]; ok {
		return fmt.Errorf("xrpc: Can not register duplicated service %v", sd.ServiceName)
	}
	newSd := *sd
	newSd.Impl = ss
	this.Services[sd.ServiceName] = &newSd
	return nil
}

func checkServiceImplType(sd *ServiceDesc, ss interface{}) error {
	if ss != nil {
		ht := reflect.TypeOf(sd.HandlerType).Elem()
		st := reflect.TypeOf(ss)
		if !st.Implements(ht) {
			return fmt.Errorf("xrpc: RegisterService found the handler of type %v that does not satisfy %v", st, ht)
		}
	} else {
		return fmt.Errorf("xrpc: Can not register nil service implementation for service %v", sd.ServiceName)
	}
	return nil
}
