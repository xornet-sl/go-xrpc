package util

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/xornet-sl/go-xrpc/xrpc/internal/xrpcpb"
	"google.golang.org/grpc/metadata"
)

type StrAddr string

func (addr StrAddr) Network() string {
	if addr != "" {
		return "tcp"
	}
	return ""
}

func (addr StrAddr) String() string {
	return string(addr)
}

func PackInvokeMetadata(md metadata.MD) *xrpcpb.Metadata {
	ret := &xrpcpb.Metadata{
		Pairs: []*xrpcpb.Metadata_Pair{},
	}
	for k, vals := range md {
		for _, v := range vals {
			ret.Pairs = append(ret.Pairs, &xrpcpb.Metadata_Pair{
				Key:   k,
				Value: v,
			})
		}
	}
	return ret
}

func UnpackInvokeMetadata(md *xrpcpb.Metadata) metadata.MD {
	if md == nil {
		return metadata.Pairs()
	}

	plainPairs := []string{}
	for _, pair := range md.Pairs {
		plainPairs = append(plainPairs, pair.GetKey())
		plainPairs = append(plainPairs, pair.GetValue())
	}

	return metadata.Pairs(plainPairs...)
}

func PackHTTPHeaders(md metadata.MD) http.Header {
	ret := http.Header{}
	for k, vv := range md {
		for _, v := range vv {
			ret.Add(k, encodeMetadataHeader(k, v))
		}
	}
	return ret
}

func UnpackHTTPHeaders(h http.Header) (metadata.MD, error) {
	if h == nil {
		return metadata.Pairs(), nil
	}

	plainPairs := []string{}
	for k, vv := range h {
		k = strings.ToLower(k)
		if !isAllowedHeader(k) {
			continue
		}
		for _, v := range vv {
			v, err := decodeMetadataHeader(k, v)
			if err != nil {
				return nil, fmt.Errorf("malformed binary metadata %q in header %q: %v", v, k, err)
			}
			plainPairs = append(plainPairs, k, v)
		}
	}
	return metadata.Pairs(plainPairs...), nil
}

func isAllowedHeader(h string) bool {
	return true
}

const binHdrSuffix = "-bin"

func encodeBinHeader(v []byte) string {
	return base64.RawStdEncoding.EncodeToString(v)
}

func decodeBinHeader(v string) ([]byte, error) {
	if len(v)%4 == 0 {
		// Input was padded, or padding was not necessary.
		return base64.StdEncoding.DecodeString(v)
	}
	return base64.RawStdEncoding.DecodeString(v)
}

func encodeMetadataHeader(k, v string) string {
	if strings.HasSuffix(k, binHdrSuffix) {
		return encodeBinHeader(([]byte)(v))
	}
	return v
}

func decodeMetadataHeader(k, v string) (string, error) {
	if strings.HasSuffix(k, binHdrSuffix) {
		b, err := decodeBinHeader(v)
		return string(b), err
	}
	return v, nil
}
