package xrpc_test

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	pb "github.com/xornet-sl/go-xrpc/proto"
	"github.com/xornet-sl/go-xrpc/xrpc"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/metadata"
)

type serverTester struct {
	srv *xrpc.RpcServer
	pb.UnimplementedPingReplyerServer
}

func (*serverTester) Ping(ctx context.Context, msg *pb.PingMessage) (*pb.PingMessage, error) {
	log.Printf("server recv Ping: %v", msg.GetMsg())
	fmt.Printf("transport: %+v\n", xrpc.GetTransportMDFromContext(ctx))
	md, _ := metadata.FromIncomingContext(ctx)
	fmt.Printf("call: %+v\n", md)
	return &pb.PingMessage{
		Msg: msg.GetMsg(),
	}, nil
}

type clientTester struct {
	pb.UnimplementedPingReplyerServer
}

func (*clientTester) Ping(ctx context.Context, msg *pb.PingMessage) (*pb.PingMessage, error) {
	log.Printf("client recv Ping: %v", msg.GetMsg())
	return &pb.PingMessage{
		Msg: msg.GetMsg(),
	}, nil
}

func TestXrpc(t *testing.T) {
	group, ctx := errgroup.WithContext(context.Background())

	//logger := configureLogger()

	group.Go(func() error { return Server(ctx, t) })
	group.Go(func() error { return Client(ctx, t) })

	if err := group.Wait(); err != nil {
		t.Error(err.Error())
	}
}

func auth(ctx context.Context, w http.ResponseWriter, r *http.Request) (context.Context, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	if md.Get("authorization") == nil {
		println("no auth")
	} else {
		fmt.Printf("auth: %q\n", md.Get("authorization")[0])
	}
	md.Set("auth", "ok")
	ctx = metadata.NewIncomingContext(ctx, md)
	return ctx, nil
}

func Server(ctx context.Context, t *testing.T) error {
	srv, err := xrpc.NewServer(
		xrpc.WithOnLogCallback(onLog),
		xrpc.WithOnDebugLogCallback(onDebugLog),
		xrpc.WithClientAuthCallback(auth),
	)
	if err != nil {
		t.Errorf("new server error: %s", err.Error())
	}
	pb.RegisterPingReplyerServer(srv, &serverTester{srv: srv})
	go func() {
		if err := srv.Serve(ctx, "localhost:9000"); err != nil {
			t.Errorf("srv.Serve err: %v", err)
		}
	}()
	return nil
}

func Client(ctx context.Context, t *testing.T) error {
	cli := xrpc.NewClient()
	pb.RegisterPingReplyerServer(cli, &clientTester{})
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "basic qwe"))
	conn, err := cli.Dial(ctx, "ws://localhost:9000", xrpc.WithOnLogCallback(onLog), xrpc.WithOnDebugLogCallback(onDebugLog))
	if err != nil {
		t.Errorf("dial error: %s", err.Error())
		return err
	}
	time.Sleep(time.Microsecond * 100)
	cli_pinger := pb.NewPingReplyerClient(conn)
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("cli-data", "qwerty"))
	resp, err := cli_pinger.Ping(ctx, &pb.PingMessage{Msg: "cli->srv"})
	if err != nil {
		t.Errorf("cli->srv err: %v", err)
	}
	t.Logf("cli->srv response: %s", resp.GetMsg())
	if !conn.IsActive() {
		t.Errorf("conn is not active")
	}
	conn.Close()
	return nil
}

func onLog(logContext *xrpc.LogContext, err error, msg string) {
	if err != nil {
		logrus.WithFields(logContext.Fields).WithError(err).Info(msg)
	} else {
		logrus.WithFields(logContext.Fields).Info(msg)
	}
}

func onDebugLog(fn xrpc.DebugLogGetter) {
	logContext, msg := fn()
	onLog(logContext, nil, fmt.Sprintf("debug: %s", msg))
}

func TestLogger(t *testing.T) {
	xrpc.NewServer(
		xrpc.WithOnLogCallback(onLog),
		xrpc.WithOnDebugLogCallback(onDebugLog),
	)
}
