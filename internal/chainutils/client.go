package chainutils

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/cometbft/cometbft/proto/tendermint/p2p"
	tmtypes "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cosmos/cosmos-sdk/client/grpc/tmservice"
	"github.com/cosmos/cosmos-sdk/codec"
	stakingTypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type QueryClient struct {
	grpcConn      *grpc.ClientConn
	stakingClient stakingTypes.QueryClient
	nodeClient    tmservice.ServiceClient
	upgradeClient upgradetypes.QueryClient
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
		nodeClient:    tmservice.NewServiceClient(grpcConn),
		upgradeClient: upgradetypes.NewQueryClient(grpcConn),
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

func (c *QueryClient) GetLatestBlock(ctx context.Context) (*tmtypes.Block, error) {
	response, err := c.nodeClient.GetLatestBlock(ctx, &tmservice.GetLatestBlockRequest{})
	if err != nil {
		return nil, err
	}
	return response.Block, nil
}

func (c *QueryClient) GetBlockHash(ctx context.Context, height int64) (string, error) {
	response, err := c.nodeClient.GetBlockByHeight(ctx, &tmservice.GetBlockByHeightRequest{
		Height: height,
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(response.BlockId.Hash), nil
}

func (c *QueryClient) IsNodeSyncing(ctx context.Context) (bool, error) {
	response, err := c.nodeClient.GetSyncing(ctx, &tmservice.GetSyncingRequest{})
	if err != nil {
		return false, err
	}
	return response.Syncing, nil
}

func (c *QueryClient) NodeInfo(ctx context.Context) (*p2p.DefaultNodeInfo, error) {
	response, err := c.nodeClient.GetNodeInfo(ctx, &tmservice.GetNodeInfoRequest{})
	if err != nil {
		return nil, err
	}
	return response.DefaultNodeInfo, nil
}

func (c *QueryClient) GetNextUpgrade(ctx context.Context) (*upgradetypes.Plan, error) {
	response, err := c.upgradeClient.CurrentPlan(ctx, &upgradetypes.QueryCurrentPlanRequest{})
	if err != nil {
		return nil, err
	}
	return response.Plan, err
}
