package chainutils

import (
	"context"
	"encoding/hex"
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/proto/tendermint/p2p"
	tmtypes "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cometbft/cometbft/rpc/client/http"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/cosmos/cosmos-sdk/client/grpc/tmservice"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/types/query"
	stakingTypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	// http client for cometbft
	rpcClient *http.HTTP

	// gRPC clients
	grpcConn      *grpc.ClientConn
	stakingClient stakingTypes.QueryClient
	nodeClient    tmservice.ServiceClient
	upgradeClient upgradetypes.QueryClient
}

func NewClient(host string) (*Client, error) {
	grpcConn, err := grpc.NewClient(
		fmt.Sprintf("%s:%d", host, GrpcPort),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(codec.NewProtoCodec(nil).GRPCCodec())),
	)
	if err != nil {
		return nil, fmt.Errorf("could not connect to grpc server at %s:%d: %w", host, GrpcPort, err)
	}

	tmClient, err := http.NewWithTimeout(
		fmt.Sprintf("http://%s:%d", host, RpcPort),
		"/websocket",
		uint(httpTimeout.Seconds()),
	)
	if err != nil {
		// Close gRPC connection to avoid leak
		grpcConn.Close()
		return nil, fmt.Errorf("could not connect to rpc server at %s:%d: %w", host, RpcPort, err)
	}

	return &Client{
		rpcClient:     tmClient,
		grpcConn:      grpcConn,
		stakingClient: stakingTypes.NewQueryClient(grpcConn),
		nodeClient:    tmservice.NewServiceClient(grpcConn),
		upgradeClient: upgradetypes.NewQueryClient(grpcConn),
	}, nil
}

func (c *Client) Close() error {
	return c.grpcConn.Close()
}

func (c *Client) QueryValidator(ctx context.Context, address string) (*stakingTypes.Validator, error) {
	response, err := c.stakingClient.Validator(ctx, &stakingTypes.QueryValidatorRequest{
		ValidatorAddr: address,
	})
	if err != nil {
		return nil, fmt.Errorf("querying validator %s: %w", address, err)
	}
	return &response.Validator, nil
}

func (c *Client) GetValidators(ctx context.Context) ([]stakingTypes.Validator, error) {
	response, err := c.stakingClient.Validators(ctx, &stakingTypes.QueryValidatorsRequest{
		Pagination: &query.PageRequest{
			Limit: paginationLimit,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("querying validators: %w", err)
	}
	return response.Validators, nil
}

func (c *Client) GetLatestBlock(ctx context.Context) (*tmtypes.Block, error) {
	response, err := c.nodeClient.GetLatestBlock(ctx, &tmservice.GetLatestBlockRequest{})
	if err != nil {
		return nil, fmt.Errorf("getting latest block: %w", err)
	}
	return response.Block, nil
}

func (c *Client) GetBlockHash(ctx context.Context, height int64) (string, error) {
	response, err := c.nodeClient.GetBlockByHeight(ctx, &tmservice.GetBlockByHeightRequest{
		Height: height,
	})
	if err != nil {
		return "", fmt.Errorf("getting block hash at height %d: %w", height, err)
	}
	return hex.EncodeToString(response.BlockId.Hash), nil
}

func (c *Client) IsNodeSyncing(ctx context.Context) (bool, error) {
	response, err := c.nodeClient.GetSyncing(ctx, &tmservice.GetSyncingRequest{})
	if err != nil {
		return false, fmt.Errorf("checking node sync status: %w", err)
	}
	return response.Syncing, nil
}

func (c *Client) NodeInfo(ctx context.Context) (*p2p.DefaultNodeInfo, error) {
	response, err := c.nodeClient.GetNodeInfo(ctx, &tmservice.GetNodeInfoRequest{})
	if err != nil {
		return nil, fmt.Errorf("getting node info: %w", err)
	}
	return response.DefaultNodeInfo, nil
}

func (c *Client) GetNextUpgrade(ctx context.Context) (*upgradetypes.Plan, error) {
	response, err := c.upgradeClient.CurrentPlan(ctx, &upgradetypes.QueryCurrentPlanRequest{})
	if err != nil {
		return nil, fmt.Errorf("getting next upgrade plan: %w", err)
	}
	return response.Plan, nil
}

func (c *Client) GetNodeStatus(ctx context.Context) (*coretypes.ResultStatus, error) {
	return c.rpcClient.Status(ctx)
}

func (c *Client) GetAbciInfo(ctx context.Context) (abci.ResponseInfo, error) {
	response, err := c.rpcClient.ABCIInfo(ctx)
	if err != nil {
		return abci.ResponseInfo{}, fmt.Errorf("getting ABCI info: %w", err)
	}
	return response.Response, nil
}
