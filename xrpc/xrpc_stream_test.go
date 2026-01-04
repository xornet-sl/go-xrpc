package xrpc_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	pb "github.com/xornet-sl/go-xrpc/proto"
	"github.com/xornet-sl/go-xrpc/xrpc"
	"golang.org/x/sync/errgroup"
)

func (this *serverTester) StreamPingsBoth(stream pb.PingReplyer_StreamPingsBothServer) error {
	for i := 0; i < 5; i++ {
		msg, err := stream.Recv()
		if err != nil {
			return err
		}
		fmt.Printf("srv received: %s\n", msg.GetMsg())
		msg.Msg = msg.GetMsg() + " - reply"
		err = stream.Send(msg)
		if err != nil {
			return err
		}
	}
	go func() {
		time.Sleep(time.Millisecond * 1000)
		this.srv.Shutdown()
	}()
	return nil
}

func TestStreams(t *testing.T) {
	group, ctx := errgroup.WithContext(context.Background())

	group.Go(func() error { return StreamServer(ctx, t) })
	group.Go(func() error { return StreamClient(ctx, t) })

	if err := group.Wait(); err != nil {
		t.Error(err.Error())
	}
}

func StreamServer(ctx context.Context, t *testing.T) error {
	srv, err := xrpc.NewServer(xrpc.WithOnLogCallback(onLog), xrpc.WithOnDebugLogCallback(onDebugLog))
	if err != nil {
		t.Errorf("new server error: %s", err.Error())
		return err
	}
	tester := &serverTester{srv: srv}
	pb.RegisterPingReplyerServer(srv, tester)
	if err := srv.ServeAndListen(ctx, "localhost:9000"); err != nil {
		t.Errorf("srv.Serve err: %v", err)
		return err
	}
	return nil
}

func StreamClient(ctx context.Context, t *testing.T) error {
	cli := xrpc.NewClient()
	conn, err := cli.Dial(ctx, "ws://localhost:9000", xrpc.WithOnLogCallback(onLog), xrpc.WithOnDebugLogCallback(onDebugLog))
	if err != nil {
		t.Errorf("dial error: %s", err.Error())
		return err
	}
	time.Sleep(time.Microsecond * 100)
	cli_pinger := pb.NewPingReplyerClient(conn)
	stream, err := cli_pinger.StreamPingsBoth(ctx)
	if err != nil {
		t.Errorf("cli->srv err: %v", err)
		return err
	}
	i := 0
	for {
		i++
		err := stream.Send(&pb.PingMessage{
			Msg: fmt.Sprintf("client->server ping %d", i),
		})
		if errors.Is(err, xrpc.StreamClosedError) {
			print("cli: server closed stream\n")
			return nil
		} else if err != nil {
			t.Errorf("cli->srv send err: %v", err)
			return err
		}
		print("ping sent\n")
		msg, err := stream.Recv()
		if errors.Is(err, xrpc.StreamClosedError) {
			print("cli: server closed stream\n")
			return nil
		} else if err != nil {
			t.Errorf("cli->srv recv err: %v", err)
			return err
		}
		fmt.Printf("cli<-srv recieved: %s\n", msg.GetMsg())
		time.Sleep(time.Millisecond * 250)
	}
}
