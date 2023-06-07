package chainutils

import (
	"context"
	"fmt"

	"github.com/cosmos/cosmos-sdk/codec"
	stakingTypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type QueryClient struct {
	grpcConn      *grpc.ClientConn
	stakingClient stakingTypes.QueryClient
}

func NewQueryClient(grpcAddress string) (*QueryClient, error) {
	grpcConn, err := grpc.Dial(
		grpcAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(codec.NewProtoCodec(nil).GRPCCodec())),
	)
	if err != nil {
		return nil, fmt.Errorf("could not connect to grpc server")
	}
	return &QueryClient{
		grpcConn:      grpcConn,
		stakingClient: stakingTypes.NewQueryClient(grpcConn),
	}, nil
}

func (c *QueryClient) Close() error {
	return c.grpcConn.Close()
}

func (c *QueryClient) QueryValidator(ctx context.Context, address string) (*stakingTypes.Validator, error) {
	response, err := c.stakingClient.Validator(ctx, &stakingTypes.QueryValidatorRequest{
		ValidatorAddr: address,
	})
	if err != nil {
		return nil, err
	}
	return &response.Validator, nil
}
