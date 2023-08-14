package main

import (
	"fmt"
	"strconv"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/descriptorpb"
)

func genServer(gen *protogen.Plugin, file *protogen.File, g *protogen.GeneratedFile, service *protogen.Service) {
	serverType := service.GoName + "Server"
	g.P("// ", serverType, " is the server API for ", service.GoName, " service.")
	g.P("// All implementations must embed Unimplemented", serverType)
	g.P("// for forward compatibility")
	if service.Desc.Options().(*descriptorpb.ServiceOptions).GetDeprecated() {
		g.P("//")
		g.P(deprecationComment)
	}
	g.Annotate(serverType, service.Location)
	g.P("type ", serverType, " interface {")
	for _, method := range service.Methods {
		g.Annotate(serverType+"."+method.GoName, method.Location)
		if method.Desc.Options().(*descriptorpb.MethodOptions).GetDeprecated() {
			g.P(deprecationComment)
		}
		g.P(method.Comments.Leading, serverSignature(g, method))
	}
	g.P("mustEmbedUnimplemented", serverType, "()")
	g.P("}")
	g.P()

	generateUnimplementedServerType(gen, file, g, service)

	if service.Desc.Options().(*descriptorpb.ServiceOptions).GetDeprecated() {
		g.P(deprecationComment)
	}
	serviceDescVar := service.GoName + "_ServiceDesc"
	g.P("func Register", service.GoName, "Server(s ", xrpcPackage.Ident("ServiceRegistrar"), ", srv ", serverType, ") {")
	g.P("s.RegisterService(&", serviceDescVar, `, srv)`)
	g.P("}")
	g.P()

	// generateServerFunctions(gen, file, g, service, serverType, serviceDescVar)
	for _, method := range service.Methods {
		if method.Desc.IsStreamingClient() || method.Desc.IsStreamingServer() {
			genServerStream(gen, file, g, method)
		} else {
			genServerMethod(gen, file, g, method)
		}
	}

	genServiceDesc(file, g, serviceDescVar, serverType, service)
}

func serverSignature(g *protogen.GeneratedFile, method *protogen.Method) string {
	var reqArgs []string
	ret := "error"
	if !method.Desc.IsStreamingClient() && !method.Desc.IsStreamingServer() {
		reqArgs = append(reqArgs, g.QualifiedGoIdent(contextPackage.Ident("Context")))
		ret = "(*" + g.QualifiedGoIdent(method.Output.GoIdent) + ", error)"
	}
	if !method.Desc.IsStreamingClient() {
		reqArgs = append(reqArgs, "*"+g.QualifiedGoIdent(method.Input.GoIdent))
	}
	if method.Desc.IsStreamingClient() || method.Desc.IsStreamingServer() {
		reqArgs = append(reqArgs, method.Parent.GoName+"_"+method.GoName+"Server")
	}
	return method.GoName + "(" + strings.Join(reqArgs, ", ") + ") " + ret
}

func generateUnimplementedServerType(gen *protogen.Plugin, file *protogen.File, g *protogen.GeneratedFile, service *protogen.Service) {
	serverType := service.GoName + "Server"
	// Server Unimplemented struct for forward compatibility.
	g.P("// Unimplemented", serverType, " must be embedded to have forward compatible implementations.")
	g.P("type Unimplemented", serverType, " struct {")
	g.P("}")
	g.P()
	for _, method := range service.Methods {
		nilArg := ""
		if !method.Desc.IsStreamingClient() && !method.Desc.IsStreamingServer() {
			nilArg = "nil,"
		}
		g.P("func (Unimplemented", serverType, ") ", serverSignature(g, method), "{")
		g.P("return ", nilArg, statusPackage.Ident("Errorf"), "(", codesPackage.Ident("Unimplemented"), `, "method `, method.GoName, ` not implemented")`)
		g.P("}")
	}
	g.P("func (Unimplemented", serverType, ") mustEmbedUnimplemented", serverType, "() {}")
	g.P()
}

func genServerMethod(gen *protogen.Plugin, file *protogen.File, g *protogen.GeneratedFile, method *protogen.Method) {
	service := method.Parent
	hname := fmt.Sprintf("_%s_%s_Handler", service.GoName, method.GoName)

	g.P("func ", hname, "(srv interface{}, ctx ", contextPackage.Ident("Context"), ", in ", protoPackage.Ident("Message"), ") (", protoPackage.Ident("Message"), ", error) {")
	g.P("return srv.(", service.GoName, "Server).", method.GoName, "(ctx, in.(*", method.Input.GoIdent, "))")
	g.P("}")
	g.P()
}

func genServerStream(gen *protogen.Plugin, file *protogen.File, g *protogen.GeneratedFile, method *protogen.Method) {
	service := method.Parent
	hname := fmt.Sprintf("_%s_%s_Handler", service.GoName, method.GoName)

	streamType := unexport(service.GoName) + method.GoName + "Server"
	g.P("func ", hname, "(srv interface{}, stream ", xrpcPackage.Ident("RpcStream"), ") error {")
	if !method.Desc.IsStreamingClient() {
		g.P("m := new(", method.Input.GoIdent, ")")
		g.P("if err := stream.RecvMsg(m); err != nil { return err }")
		g.P("return srv.(", service.GoName, "Server).", method.GoName, "(m, &", streamType, "{stream})")
	} else {
		g.P("return srv.(", service.GoName, "Server).", method.GoName, "(&", streamType, "{stream})")
	}
	g.P("}")
	g.P()

	genSend := method.Desc.IsStreamingServer()
	genSendAndClose := !method.Desc.IsStreamingServer()
	genRecv := method.Desc.IsStreamingClient()

	// Stream auxiliary types and methods.
	g.P("type ", service.GoName, "_", method.GoName, "Server interface {")
	if genSend {
		g.P("Send(*", method.Output.GoIdent, ") error")
	}
	if genSendAndClose {
		g.P("SendAndClose(*", method.Output.GoIdent, ") error")
	}
	if genRecv {
		g.P("Recv() (*", method.Input.GoIdent, ", error)")
	}
	g.P(xrpcPackage.Ident("RpcStream"))
	g.P("}")
	g.P()

	g.P("type ", streamType, " struct {")
	g.P(xrpcPackage.Ident("RpcStream"))
	g.P("}")
	g.P()

	if genSend {
		g.P("func (x *", streamType, ") Send(m *", method.Output.GoIdent, ") error {")
		g.P("return x.RpcStream.SendMsg(m)")
		g.P("}")
		g.P()
	}
	if genSendAndClose {
		g.P("func (x *", streamType, ") SendAndClose(m *", method.Output.GoIdent, ") error {")
		g.P("return x.RpcStream.SendMsg(m)")
		g.P("}")
		g.P()
	}
	if genRecv {
		g.P("func (x *", streamType, ") Recv() (*", method.Input.GoIdent, ", error) {")
		g.P("m := new(", method.Input.GoIdent, ")")
		g.P("if err := x.RpcStream.RecvMsg(m); err != nil { return nil, err }")
		g.P("return m, nil")
		g.P("}")
		g.P()
	}
}

func genServiceDesc(file *protogen.File, g *protogen.GeneratedFile, serviceDescVar string, serverType string, service *protogen.Service) {
	// Service descriptor.
	g.P("// ", serviceDescVar, " is the ", xrpcPackage.Ident("ServiceDesc"), " for ", service.GoName, " service.")
	g.P("// It's only intended for direct use with ", xrpcPackage.Ident("RegisterService"), ",")
	g.P("// and not to be introspected or modified (even as a copy)")
	g.P("var ", serviceDescVar, " = ", xrpcPackage.Ident("ServiceDesc"), " {")
	g.P("ServiceName: ", strconv.Quote(string(service.Desc.FullName())), ",")
	g.P("HandlerType: (*", serverType, ")(nil),")
	g.P("Methods: map[string]", xrpcPackage.Ident("MethodHandler"), "{")
	for _, method := range service.Methods {
		if method.Desc.IsStreamingClient() || method.Desc.IsStreamingServer() {
			continue
		}
		hname := fmt.Sprintf("_%s_%s_Handler", service.GoName, method.GoName)
		g.P(strconv.Quote(string(method.Desc.Name())), ": ", hname, ",")
	}
	g.P("},")

	g.P("Streams: map[string]", xrpcPackage.Ident("StreamDesc"), "{")
	for _, method := range service.Methods {
		if !method.Desc.IsStreamingClient() && !method.Desc.IsStreamingServer() {
			continue
		}
		hname := fmt.Sprintf("_%s_%s_Handler", service.GoName, method.GoName)
		g.P(strconv.Quote(string(method.Desc.Name())), ": {")
		g.P("Handler: ", hname, ",")
		if method.Desc.IsStreamingServer() {
			g.P("ServerStreams: true,")
		}
		if method.Desc.IsStreamingClient() {
			g.P("ClientStreams: true,")
		}
		g.P("},")
	}
	g.P("},")

	g.P("}")
	g.P()
}
