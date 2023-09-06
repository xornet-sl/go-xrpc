package main

import (
	"context"
	"fmt"
	"math"
	"sync/atomic"
	"time"

	"github.com/fatih/color"
	"github.com/rodaine/table"
	"github.com/xornet-sl/go-xrpc/xrpc"
)

type stats struct {
	RpcClient, RpcServer struct {
		CallOut   atomic.Uint64
		CallIn    atomic.Uint64
		StreamOut atomic.Uint64
		StreamIn  atomic.Uint64
		stream    atomic.Value // xrpc.RpcStream
	}
}

var Stats stats

func (s *stats) Clear() {
	s.RpcClient.CallIn.Store(0)
	s.RpcClient.CallOut.Store(0)
	s.RpcClient.StreamIn.Store(0)
	s.RpcClient.StreamOut.Store(0)
	s.RpcServer.CallIn.Store(0)
	s.RpcServer.CallOut.Store(0)
	s.RpcServer.StreamIn.Store(0)
	s.RpcServer.StreamOut.Store(0)
}

func (s *stats) Display(ctx context.Context) {
	headerFmt := color.New(color.FgGreen, color.Underline).SprintfFunc()
	columnFmt := color.New(color.FgYellow).SprintfFunc()

	tbl := table.New("RpcDirection", "CallIn", "CallOut", "StreamIn", "StreamOut", "InQueueLen")
	tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)

	var lastClientCallIn, lastClientCallOut, lastClientStreamIn, lastClientStreamOut uint64
	var lastServerCallIn, lastServerCallOut, lastServerStreamIn, lastServerStreamOut uint64
	var lastTimestamp time.Time

	storeLast := func() {
		lastClientCallIn = s.RpcClient.CallIn.Load()
		lastClientCallOut = s.RpcClient.CallOut.Load()
		lastClientStreamIn = s.RpcClient.StreamIn.Load()
		lastClientStreamOut = s.RpcClient.StreamOut.Load()
		lastServerCallIn = s.RpcServer.CallIn.Load()
		lastServerCallOut = s.RpcServer.CallOut.Load()
		lastServerStreamIn = s.RpcServer.StreamIn.Load()
		lastServerStreamOut = s.RpcServer.StreamOut.Load()
		lastTimestamp = time.Now()
	}

	for {
		storeLast()

		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second): // TODO: custom interval
		}

		window := time.Since(lastTimestamp)
		clientInQueueLen := "*"
		serverInQueueLen := "*"
		if stream := s.RpcClient.stream.Load(); stream != nil {
			clientInQueueLen = s.formatSize(stream.(xrpc.RpcStream).GetIncomingQueueLen())
		}
		if stream := s.RpcServer.stream.Load(); stream != nil {
			serverInQueueLen = s.formatSize(stream.(xrpc.RpcStream).GetIncomingQueueLen())
		}

		tbl.SetRows([][]string{
			{
				"client",
				s.calcDiff(lastClientCallIn, s.RpcClient.CallIn.Load(), window),
				s.calcDiff(lastClientCallOut, s.RpcClient.CallOut.Load(), window),
				s.calcDiff(lastClientStreamIn, s.RpcClient.StreamIn.Load(), window),
				s.calcDiff(lastClientStreamOut, s.RpcClient.StreamOut.Load(), window),
				clientInQueueLen,
			},
			{
				"server",
				s.calcDiff(lastServerCallIn, s.RpcServer.CallIn.Load(), window),
				s.calcDiff(lastServerCallOut, s.RpcServer.CallOut.Load(), window),
				s.calcDiff(lastServerStreamIn, s.RpcServer.StreamIn.Load(), window),
				s.calcDiff(lastServerStreamOut, s.RpcServer.StreamOut.Load(), window),
				serverInQueueLen,
			},
		})
		tbl.Print()
	}
}

func (s *stats) formatSize(size int64) string {
	var bits = false // TODO: customize

	fSize := float64(size)

	BLetter := "B"
	if bits {
		fSize *= 8
		BLetter = "b"
	}

	bandLetter := ""
	bands := []string{"k", "M", "G", "T"}

	for _, band := range bands {
		if fSize < 1000 {
			break
		}
		fSize /= 1000
		bandLetter = band
	}

	if bandLetter != "" {
		return fmt.Sprintf("%.2f %s%s", fSize, bandLetter, BLetter)
	} else {
		return fmt.Sprintf("%.f %s%s", fSize, bandLetter, BLetter)
	}
}

func (s *stats) calcDiff(old uint64, new uint64, window time.Duration) string {
	var bits = false // TODO: customize

	diff := float64(new - old)
	diff /= window.Seconds()
	BLetter := "B"
	if bits {
		diff *= 8
		BLetter = "b"
	}

	bandLetter := ""
	bands := []string{"k", "M", "G", "T"}

	for _, band := range bands {
		if diff < 1000 {
			break
		}
		diff /= 1000
		bandLetter = band
	}

	if bandLetter == "G" || bandLetter == "T" {
		return fmt.Sprintf("%.2f %s%s/s", diff, bandLetter, BLetter)
	} else {
		return fmt.Sprintf("%.f %s%s/s", math.Round(diff), bandLetter, BLetter)
	}
}
