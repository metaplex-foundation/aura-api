package proto

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	maxMessageSizeBytes = 1024 * 1024 * 50 // 50mb
)

func NewClient(addr string) (AuraClient, error) {
	conn, err := grpc.Dial(
		addr,
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxMessageSizeBytes), grpc.MaxCallSendMsgSize(maxMessageSizeBytes)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	return NewAuraClient(conn), nil
}
