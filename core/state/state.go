package state

import (
	"github.com/MinterTeam/minter-go-node/core/state/accounts"
	"github.com/MinterTeam/minter-go-node/core/state/app"
	"github.com/MinterTeam/minter-go-node/core/state/candidates"
	"github.com/MinterTeam/minter-go-node/core/state/checks"
	"github.com/MinterTeam/minter-go-node/core/state/coins"
	"github.com/MinterTeam/minter-go-node/core/state/frozen_funds"
	"github.com/MinterTeam/minter-go-node/core/state/validators"
	"github.com/MinterTeam/minter-go-node/core/types"
	"github.com/MinterTeam/minter-go-node/tree"
	db "github.com/tendermint/tm-db"
)

type State struct {
	App         *app.App
	Validators  *validators.Validators
	Candidates  *candidates.Candidates
	FrozenFunds *frozen_funds.FrozenFunds
	Accounts    *accounts.Accounts
	Coins       *coins.Coins
	Checks      *checks.Checks

	height uint64
	tree   tree.Tree
}

func NewState(height uint64, db db.DB) (*State, error) {
	iavlTree, err := tree.NewMutableTree(db).GetImmutableAtHeight(int64(height))
	if err != nil {
		return nil, err
	}

	validatorsState, err := validators.NewValidators(db)
	if err != nil {
		return nil, err
	}

	candidatesState, err := candidates.NewCandidates(iavlTree)
	if err != nil {
		return nil, err
	}

	appState, err := app.NewApp(iavlTree)
	if err != nil {
		return nil, err
	}

	frozenFundsState, err := frozen_funds.NewFrozenFunds(db)
	if err != nil {
		return nil, err
	}

	accountsState, err := accounts.NewAccounts(iavlTree)
	if err != nil {
		return nil, err
	}

	coinsState, err := coins.NewCoins(iavlTree)
	if err != nil {
		return nil, err
	}

	checksState, err := checks.NewChecks(db)
	if err != nil {
		return nil, err
	}

	state := &State{
		Validators:  validatorsState,
		App:         appState,
		Candidates:  candidatesState,
		FrozenFunds: frozenFundsState,
		Accounts:    accountsState,
		Coins:       coinsState,
		Checks:      checksState,

		height: height,
		tree:   iavlTree,
	}

	return state, nil
}

func NewCheckState(state *State) *State {
	panic("implement me")
}

func NewCheckStateAtHeight(height uint64, db db.DB) (*State, error) {
	panic("implement me")
}

func (s *State) Commit() ([]byte, error) {
	if err := s.Accounts.Commit(); err != nil {
		return nil, err
	}

	if err := s.App.Commit(); err != nil {
		return nil, err
	}

	hash, _, err := s.tree.SaveVersion()

	return hash, err
}

func (s *State) CheckForInvariants() error {
	panic("implement me")
}

func (s *State) Height() uint64 {
	return s.height
}

func (s *State) Import(state types.AppState) error {
	panic("implement me")
}

func (s *State) Export(height uint64) types.AppState {
	panic("implement me")
}
