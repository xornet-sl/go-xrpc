package main

import (
	"context"
	"errors"
	"os"
	"sync/atomic"
	"time"

	prefixed "github.com/hu13/logrus-prefixed-formatter"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/xornet-sl/go-xrpc/cmd/xrpc-loadtest/proto"
	"github.com/xornet-sl/go-xrpc/xrpc"
)

type config struct {
	Listen    string
	ConnectTo string

	Calls struct {
		SendInterval     time.Duration
		SendMessageSize  uint
		ReplyMessageSize uint
		ReplyDelay       time.Duration
	}

	Streams struct {
		SendInterval     time.Duration
		SendMessageSize  uint
		ReceiveDelay     time.Duration
		ReplyInterval    time.Duration
		ReplyMessageSize uint
	}
}

var (
	Cfg config

	Rpc struct {
		client proto.LoadTestClient
		server LoadTestServer
	}

	hasClient atomic.Bool
)

type LoadTestServer struct {
	proto.UnimplementedLoadTestServer
}

func client(ctx context.Context, opts []xrpc.Option) error {
	client := xrpc.NewClient()
	proto.RegisterLoadTestServer(client, &Rpc.server)
	conn, err := client.Dial(ctx, Cfg.ConnectTo, opts...)
	if err != nil {
		return err
	}
	Rpc.client = proto.NewLoadTestClient(conn)

	return DoRpcs(conn.Context(), conn)
}

func server(ctx context.Context, opts []xrpc.Option) error {
	server, err := xrpc.NewServer(opts...)
	if err != nil {
		return err
	}
	proto.RegisterLoadTestServer(server, &Rpc.server)

	logrus.Infof("Listening on %s", Cfg.Listen)
	return server.ServeAndListen(ctx, Cfg.Listen)
}

func onOpen(ctx context.Context, conn *xrpc.RpcConn) (context.Context, error) {
	if Cfg.IsServer() {
		if !hasClient.CompareAndSwap(false, true) {
			return nil, errors.New("Multiple clients are not supported.")
		}
		Rpc.client = proto.NewLoadTestClient(conn)
		Stats.Clear()
		logrus.WithField("remote", conn.RemoteAddr().String()).Info("New client connected")
		go func() {
			_ = DoRpcs(ctx, conn)
		}()
	} else {
		Stats.Clear()
		logrus.Info("Connection established")
	}
	go Stats.Display(ctx)

	return nil, nil
}

func onClose(ctx context.Context, conn *xrpc.RpcConn, closeError error) {
	if Cfg.IsServer() {
		hasClient.Store(false)
		logrus.WithField("remote", conn.RemoteAddr().String()).Info("Client disconnected")
	} else {
		logrus.WithError(closeError).Info("Connection closed")
	}
}

func root() error {
	opts := []xrpc.Option{
		xrpc.WithConnOpenCallback(onOpen),
		xrpc.WithConnClosedCallback(onClose),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if Cfg.IsServer() {
		return server(ctx, opts)
	} else {
		return client(ctx, opts)
	}
}

var rootCmd = &cobra.Command{
	Use: os.Args[0],
	RunE: func(cmd *cobra.Command, args []string) error {
		if Cfg.ConnectTo == "" && Cfg.Listen == "" {
			return errors.New("Either --connect or --listen must be specified")
		}

		if err := root(); err != nil {
			println(err.Error())
			os.Exit(1)
		}

		return nil
	},
}

func main() {
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetFormatter(&prefixed.TextFormatter{
		TimestampFormat: "2006-01-02 15:04:05.000",
		FullTimestamp:   true,
		ForceFormatting: true,
	})

	rootCmd.Flags().StringVarP(&Cfg.Listen, "listen", "l", "", "socket to listen")
	rootCmd.Flags().StringVarP(&Cfg.ConnectTo, "connect", "c", "", "connect to")
	rootCmd.MarkFlagsMutuallyExclusive("listen", "connect")

	rootCmd.Flags().DurationVar(&Cfg.Calls.SendInterval, "call-send-interval", 0, "calls send interval")
	rootCmd.Flags().UintVar(&Cfg.Calls.SendMessageSize, "call-send-size", 0, "call send message size")
	rootCmd.Flags().UintVar(&Cfg.Calls.ReplyMessageSize, "call-reply-size", 0, "call reply message size")
	rootCmd.Flags().DurationVar(&Cfg.Calls.ReplyDelay, "call-reply-delay", 0, "call replay delay")

	rootCmd.Flags().DurationVar(&Cfg.Streams.SendInterval, "stream-send-interval", 0, "stream send interval")
	rootCmd.Flags().UintVar(&Cfg.Streams.SendMessageSize, "stream-send-size", 0, "stream send message size")
	rootCmd.Flags().DurationVar(&Cfg.Streams.ReceiveDelay, "stream-receive-delay", 0, "stream receive delay")
	rootCmd.Flags().DurationVar(&Cfg.Streams.ReplyInterval, "stream-reply-interval", 0, "stream reply interval")
	rootCmd.Flags().UintVar(&Cfg.Streams.ReplyMessageSize, "stream-reply-size", 0, "stream reply message size")

	if rootCmd.Execute() != nil {
		os.Exit(1)
	}
}

func (c *config) IsServer() bool {
	return c.Listen != ""
}
