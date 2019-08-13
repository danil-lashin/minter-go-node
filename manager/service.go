package manager

import (
	"context"
	"github.com/MinterTeam/minter-go-node/config"
	"github.com/MinterTeam/minter-go-node/core/minter"
	"github.com/MinterTeam/minter-go-node/manager/manager"
	rpc "github.com/tendermint/tendermint/rpc/client"
)

type Manager struct {
	blockchain *minter.Blockchain
	tmRPC      *rpc.Local
	cfg        *config.Config
}

func (s *Manager) ExportStates(context context.Context, request *manager.ExportStatesRequest) (*manager.ExportStatesReply, error) {
	err := s.blockchain.ExportStates(request.FromHeight, request.ToHeight)
	if err != nil {
		return nil, err
	}
	return &manager.ExportStatesReply{
		Success: true,
	}, nil
}

func (s *Manager) PruneStates(context context.Context, request *manager.PruneStatesRequest) (*manager.PruneStatesReply, error) {
	s.blockchain.PruneStates(request.FromHeight, request.ToHeight)
	return &manager.PruneStatesReply{
		Success: true,
	}, nil
}

func (s *Manager) GetStatus(context.Context, *manager.StatusRequest) (*manager.StatusReply, error) {
	result, err := s.tmRPC.Status()
	if err != nil {
		return nil, err
	}

	return &manager.StatusReply{
		Height: result.SyncInfo.LatestBlockHeight,
	}, nil
}

func NewManager(blockchain *minter.Blockchain, tmRPC *rpc.Local, cfg *config.Config) *Manager {
	return &Manager{blockchain: blockchain, tmRPC: tmRPC, cfg: cfg}
}
