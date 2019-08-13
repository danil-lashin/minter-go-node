package manager

import (
	"github.com/MinterTeam/minter-go-node/cmd/utils"
	"github.com/MinterTeam/minter-go-node/config"
	"github.com/MinterTeam/minter-go-node/core/minter"
	"github.com/MinterTeam/minter-go-node/manager/manager"
	rpc "github.com/tendermint/tendermint/rpc/client"
	"google.golang.org/grpc"
	"net"
)

func Run(blockchain *minter.Blockchain, tmRPC *rpc.Local, cfg *config.Config, stop chan bool) {
	lis, err := net.Listen("unix", utils.GetManagerSocket())
	if err != nil {
		panic(err)
	}

	grpcServer := grpc.NewServer()
	manager.RegisterManagerServer(grpcServer, NewManager(blockchain, tmRPC, cfg))

	go func() {
		<-stop
		grpcServer.Stop()
	}()

	err = grpcServer.Serve(lis)
	if err != nil {
		panic(err)
	}
}
