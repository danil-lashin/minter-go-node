package cmd

import (
	"context"
	"encoding/json"
	"github.com/MinterTeam/minter-go-node/cmd/utils"
	"github.com/MinterTeam/minter-go-node/manager/manager"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"os"
	"strconv"
)

var Manager = &cobra.Command{
	Use:   "manager",
	Short: "Node's manager",
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := grpc.Dial("unix://"+utils.GetManagerSocket(), grpc.WithInsecure())
		if err != nil {
			panic(err)
		}
		defer conn.Close()

		client := manager.NewManagerClient(conn)

		var response interface{}

		switch args[0] {
		case "status":
			response, err = client.GetStatus(context.Background(), &manager.StatusRequest{})
		case "prune_states":
			fromHeight, err := strconv.Atoi(args[1])
			if err != nil {
				panic(err)
			}
			toHeight, err := strconv.Atoi(args[2])
			if err != nil {
				panic(err)
			}
			response, err = client.PruneStates(context.Background(), &manager.PruneStatesRequest{
				FromHeight: int64(fromHeight),
				ToHeight:   int64(toHeight),
			})
		default:
			println("Command not found")
			os.Exit(1)
		}

		if err != nil {
			panic(err)
		}
		data, err := json.MarshalIndent(response, "", "  ")
		if err != nil {
			panic(err)
		}

		println(string(data))
		return nil
	},
}
