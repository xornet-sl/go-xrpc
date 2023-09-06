package main

import (
	"context"
	"time"

	"github.com/xornet-sl/go-xrpc/cmd/xrpc-loadtest/proto"
	"github.com/xornet-sl/go-xrpc/xrpc"
	"golang.org/x/sync/errgroup"
)

func (s *LoadTestServer) Call(ctx context.Context, msg *proto.Message) (*proto.Message, error) {
	Stats.RpcServer.CallIn.Add(uint64(len(msg.GetMsg())))

	if Cfg.Calls.ReplyMessageSize == 0 {
		msg.Msg = []byte{} // Empty result
		return msg, nil
	}

	if Cfg.Calls.ReplyDelay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(Cfg.Calls.ReplyDelay):
		}
	}

	msg.Msg = genPayload(Cfg.Calls.ReplyMessageSize)

	Stats.RpcServer.CallOut.Add(uint64(Cfg.Calls.ReplyMessageSize))
	return msg, nil
}

func (s *LoadTestServer) Stream(stream proto.LoadTest_StreamServer) error {
	group, ctx := errgroup.WithContext(stream.Context())

	Stats.RpcServer.stream.Store(stream)
	defer Stats.RpcServer.stream.Store(nil)

	group.Go(func() error {
		for {
			msg, err := stream.Recv()
			if err != nil {
				return err
			}
			Stats.RpcServer.StreamIn.Add(uint64(len(msg.GetMsg())))

			if Cfg.Streams.ReceiveDelay > 0 {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(Cfg.Streams.ReceiveDelay):
				}
			}
		}
	})

	if Cfg.Streams.ReplyMessageSize > 0 && Cfg.Streams.ReplyInterval > 0 {
		group.Go(func() error {
			for {
				Stats.RpcServer.StreamOut.Add(uint64(Cfg.Streams.ReplyMessageSize))
				if err := stream.Send(&proto.Message{
					Msg: genPayload(Cfg.Streams.ReplyMessageSize),
				}); err != nil {
					return err
				}

				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(Cfg.Streams.ReplyInterval):
				}
			}
		})
	} else {
		stream.CloseSend()
	}

	return group.Wait()
}

func DoCalls(ctx context.Context, conn *xrpc.RpcConn) error {
	if Cfg.Calls.SendInterval == 0 || Cfg.Calls.SendMessageSize == 0 {
		return nil
	}

	cli := proto.NewLoadTestClient(conn)
	for {
		Stats.RpcClient.CallOut.Add(uint64(Cfg.Calls.SendMessageSize))
		ret, err := cli.Call(ctx, &proto.Message{
			Msg: genPayload(Cfg.Calls.SendMessageSize),
		})
		if err != nil {
			return err
		}
		Stats.RpcClient.CallIn.Add(uint64(len(ret.GetMsg())))

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(Cfg.Calls.SendInterval):
		}
	}
}

func DoStream(ctx context.Context, conn *xrpc.RpcConn) error {
	cli := proto.NewLoadTestClient(conn)
	stream, err := cli.Stream(ctx)
	if err != nil {
		return err
	}
	Stats.RpcClient.stream.Store(stream)
	defer Stats.RpcClient.stream.Store(nil)

	group, ctx := errgroup.WithContext(stream.Context())

	group.Go(func() error {
		for {
			msg, err := stream.Recv()
			if err != nil {
				return err
			}
			Stats.RpcClient.StreamIn.Add(uint64(len(msg.GetMsg())))

			if Cfg.Streams.ReceiveDelay > 0 {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(Cfg.Streams.ReceiveDelay):
				}
			}
		}
	})

	if Cfg.Streams.SendMessageSize > 0 && Cfg.Streams.SendInterval > 0 {
		group.Go(func() error {
			for {
				Stats.RpcClient.StreamOut.Add(uint64(Cfg.Streams.SendMessageSize))
				if err := stream.Send(&proto.Message{
					Msg: genPayload(Cfg.Streams.SendMessageSize),
				}); err != nil {
					return err
				}

				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(Cfg.Streams.SendInterval):
				}
			}
		})
	} else {
		stream.CloseSend()
	}

	return group.Wait()
}

func DoRpcs(ctx context.Context, conn *xrpc.RpcConn) error {
	group, ctx := errgroup.WithContext(ctx)

	group.Go(func() error { return DoCalls(ctx, conn) })
	group.Go(func() error { return DoStream(ctx, conn) })

	return group.Wait()
}

func genPayload(length uint) []byte {
	ret := make([]byte, length)
	return ret
}
