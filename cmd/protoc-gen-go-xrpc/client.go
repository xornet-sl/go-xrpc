package main

import (
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/descriptorpb"
)

func genClient(gen *protogen.Plugin, file *protogen.File, g *protogen.GeneratedFile, service *protogen.Service) {
	clientName := service.GoName + "Client"
	g.P("// ", clientName, " is the client API for ", service.GoName, " service.")
	if service.Desc.Options().(*descriptorpb.ServiceOptions).GetDeprecated() {
		g.P("//")
		g.P(deprecationComment)
	}
	g.Annotate(clientName, service.Location)
	g.P("type ", clientName, " interface {")
	for _, method := range service.Methods {
		g.Annotate(clientName+"."+method.GoName, method.Location)
		if method.Desc.Options().(*descriptorpb.MethodOptions).GetDeprecated() {
			g.P(deprecationComment)
		}
		g.P(method.Comments.Leading, clientMethodSignature(g, method))
	}
	g.P("}")
	g.P()

	clientStruct(g, clientName)

	if service.Desc.Options().(*descriptorpb.ServiceOptions).GetDeprecated() {
		g.P(deprecationComment)
	}
	g.P("func New", clientName, " (cc ", xrpcPackage.Ident("InvokableConnection"), ") ", clientName, " {")
	g.P("return &", unexport(clientName), "{cc}")
	g.P("}")
	g.P()

	for _, method := range service.Methods {
		genClientMethod(gen, file, g, method)
	}
}

func clientMethodSignature(g *protogen.GeneratedFile, method *protogen.Method) string {
	s := method.GoName + "(ctx " + g.QualifiedGoIdent(contextPackage.Ident("Context"))
	if !method.Desc.IsStreamingClient() {
		s += ", in *" + g.QualifiedGoIdent(method.Input.GoIdent)
	}
	s += ", opts ..." + g.QualifiedGoIdent(xrpcPackage.Ident("CallOption")) + ") ("
	if !method.Desc.IsStreamingClient() && !method.Desc.IsStreamingServer() {
		s += "*" + g.QualifiedGoIdent(method.Output.GoIdent)
	} else {
		s += method.Parent.GoName + "_" + method.GoName + "Client"
	}
	s += ", error)"
	return s
}

func clientStruct(g *protogen.GeneratedFile, clientName string) {
	g.P("type ", unexport(clientName), " struct {")
	g.P("cc ", xrpcPackage.Ident("InvokableConnection"))
	g.P("}")
	g.P()
}

func genClientMethod(gen *protogen.Plugin, file *protogen.File, g *protogen.GeneratedFile, method *protogen.Method) {
	service := method.Parent
	if method.Desc.Options().(*descriptorpb.MethodOptions).GetDeprecated() {
		g.P(deprecationComment)
	}
	g.P("func (c *", unexport(service.GoName), "Client) ", clientMethodSignature(g, method), "{")
	if !method.Desc.IsStreamingServer() && !method.Desc.IsStreamingClient() {
		g.P("out := new(", method.Output.GoIdent, ")")
		g.P(`err := c.cc.Invoke(ctx, "`, service.Desc.FullName(), `", "`, method.GoName, `", in, out, opts...)`)
		g.P("if err != nil { return nil, err }")
		g.P("return out, nil")
		g.P("}")
		g.P()
		return
	}

	// Streams
	streamType := unexport(service.GoName) + method.GoName + "Client"
	g.P(`stream, err := c.cc.NewStream(ctx, "`, service.Desc.FullName(), `", "`, method.GoName, `", opts...)`)
	g.P("if err != nil { return nil, err }")
	g.P("x := &", streamType, "{stream}")
	if !method.Desc.IsStreamingClient() {
		g.P("if err := x.RpcStream.SendMsg(in); err != nil { return nil, err }")
		g.P("if err := x.RpcStream.CloseSend(); err != nil { return nil, err }")
	}
	g.P("return x, nil")
	g.P("}")
	g.P()

	genSend := method.Desc.IsStreamingClient()
	genRecv := method.Desc.IsStreamingServer()
	genCloseAndRecv := !method.Desc.IsStreamingServer()

	// Stream auxiliary types and methods.
	g.P("type ", service.GoName, "_", method.GoName, "Client interface {")
	if genSend {
		g.P("Send(*", method.Input.GoIdent, ") error")
	}
	if genRecv {
		g.P("Recv() (*", method.Output.GoIdent, ", error)")
	}
	if genCloseAndRecv {
		g.P("CloseAndRecv() (*", method.Output.GoIdent, ", error)")
	}
	g.P(xrpcPackage.Ident("RpcStream"))
	g.P("}")
	g.P()

	g.P("type ", streamType, " struct {")
	g.P(xrpcPackage.Ident("RpcStream"))
	g.P("}")
	g.P()

	if genSend {
		g.P("func (x *", streamType, ") Send(m *", method.Input.GoIdent, ") error {")
		g.P("return x.RpcStream.SendMsg(m)")
		g.P("}")
		g.P()
	}
	if genRecv {
		g.P("func (x *", streamType, ") Recv() (*", method.Output.GoIdent, ", error) {")
		g.P("m := new(", method.Output.GoIdent, ")")
		g.P("if err := x.RpcStream.RecvMsg(m); err != nil { return nil, err }")
		g.P("return m, nil")
		g.P("}")
		g.P()
	}
	if genCloseAndRecv {
		g.P("func (x *", streamType, ") CloseAndRecv() (*", method.Output.GoIdent, ", error) {")
		g.P("if err := x.RpcStream.CloseSend(); err != nil { return nil, err }")
		g.P("m := new(", method.Output.GoIdent, ")")
		g.P("if err := x.RpcStream.RecvMsg(m); err != nil { return nil, err }")
		g.P("return m, nil")
		g.P("}")
		g.P()
	}
}
