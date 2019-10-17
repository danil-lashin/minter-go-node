package candidates

import (
	"fmt"
	"github.com/MinterTeam/minter-go-node/core/state/bus"
	"github.com/MinterTeam/minter-go-node/core/types"
	"github.com/MinterTeam/minter-go-node/formula"
	"github.com/MinterTeam/minter-go-node/rlp"
	"github.com/MinterTeam/minter-go-node/tree"
	"math/big"
)

const (
	CandidateStatusOffline = 0x01
	CandidateStatusOnline  = 0x02

	UnbondPeriod              = 518400
	MaxDelegatorsPerCandidate = 1000

	mainPrefix        = 'c'
	stakesPrefix      = 's'
	stakesStatePrefix = 'q'
	totalStakePrefix  = 't'
)

type Candidates struct {
	list   map[types.Pubkey]*Candidate
	loaded bool

	iavl tree.Tree
	bus  *bus.Bus
}

func NewCandidates(iavl tree.Tree, bus *bus.Bus) (*Candidates, error) {
	candidates := &Candidates{iavl: iavl, bus: bus}
	candidates.bus.SetCandidates(NewBus(candidates))

	return candidates, nil
}

func (c *Candidates) Commit() error {
	if !c.loaded {
		return nil
	}

	// todo commit

	return nil
}

func (c *Candidates) GetNewCandidates(valCount int, height int64) []Candidate {
	panic("implement me")
}

func (c *Candidates) PunishByzantineCandidate(height uint64, tmAddress types.TmAddress) {
	candidate := c.GetCandidateByTendermintAddress(tmAddress)
	stakes := c.GetStakes(candidate.PubKey)

	for _, stake := range stakes {
		newValue := big.NewInt(0).Set(stake.Value)
		newValue.Mul(newValue, big.NewInt(95))
		newValue.Div(newValue, big.NewInt(100))

		slashed := big.NewInt(0).Set(stake.Value)
		slashed.Sub(slashed, newValue)

		if !stake.Coin.IsBaseCoin() {
			coin := c.bus.Coins().GetCoin(stake.Coin)
			ret := formula.CalculateSaleReturn(coin.Volume, coin.Reserve, coin.Crr, slashed)

			c.bus.Coins().SubCoinVolume(coin.Symbol, slashed)
			c.bus.Coins().SubCoinReserve(coin.Symbol, ret)

			c.bus.App().AddTotalSlashed(ret)
		} else {
			c.bus.App().AddTotalSlashed(slashed)
		}

		// todo: add event
		//edb.AddEvent(s.height, events.SlashEvent{
		//	Address:         stake.Owner,
		//	Amount:          slashed.Bytes(),
		//	Coin:            stake.Coin,
		//	ValidatorPubKey: candidate.PubKey,
		//})

		c.bus.FrozenFunds().AddFrozenFund(height+UnbondPeriod, stake.Owner, candidate.PubKey, stake.Coin, newValue)
		c.bus.Coins().SanitizeCoin(stake.Coin)
	}
}

func (c *Candidates) GetCandidateByTendermintAddress(address types.TmAddress) *Candidate {
	c.loadCandidates()

	candidates := c.GetCandidates()
	for _, candidate := range candidates {
		if candidate.GetTmAddress() == address {
			return candidate
		}
	}

	return nil
}

func (c *Candidates) RecalculateStakes() {
	c.loadCandidates()

	for _, candidate := range c.list {
		stakes := c.GetStakes(candidate.PubKey)

		for _, stake := range stakes {
			stake.setBipValue(c.calculateBipValue(stake.Coin, stake.Value, false, true))
		}

		for _, update := range candidate.updates {
			bipValue := c.calculateBipValue(update.Coin, update.Value, false, true)
			stake := c.GetStakeOfAddress(candidate.PubKey, update.Owner, update.Coin)
			if stake == nil {
				state := c.getStakeState(candidate.PubKey)
				i := 0
				currentIndex := state.Tail
				for i < state.Count && currentIndex != -1 {
					stake := c.getStakeAtIndex(candidate.PubKey, currentIndex)
					currentIndex = stake.PrevStakeIndex

					if stake.BipValue.Cmp(bipValue) == -1 {
						c.bus.Accounts().AddBalance(update.Owner, update.Coin, update.Value)
						// todo: unbond event

						stake.setNewOwner(stake.Coin, stake.Owner)
						break
					}
				}
			}

			if stake != nil {
				stake.addValue(update.Value)
				update.Value = big.NewInt(0)
				stake.setBipValue(c.calculateBipValue(stake.Coin, stake.Value, false, true))
			}
		}
		candidate.clearUpdates()

		totalBipValue := big.NewInt(0)
		for _, stake := range stakes {
			totalBipValue.Add(totalBipValue, stake.BipValue)
		}

		candidate.setTotalBipValue(totalBipValue)
	}
}

func (c *Candidates) Exists(pubkey types.Pubkey) bool {
	c.loadCandidates()
	_, exists := c.list[pubkey]

	return exists
}

func (c *Candidates) Count() int {
	c.loadCandidates()

	return len(c.list)
}

func (c *Candidates) IsNewCandidateStakeSufficient(coin types.CoinSymbol, stake *big.Int) bool {
	bipValue := c.calculateBipValue(coin, stake, true, true)
	candidates := c.list

	for _, candidate := range candidates {
		if candidate.totalBipStake.Cmp(bipValue) == -1 {
			return true
		}
	}

	return false
}

func (c *Candidates) Create(ownerAddress types.Address, rewardAddress types.Address, pubkey types.Pubkey, commission uint, coin types.CoinSymbol, stake *big.Int) {
	panic("implement me")
}

func (c *Candidates) GetCandidate(pubkey types.Pubkey) *Candidate {
	c.loadCandidates()

	return c.list[pubkey]
}

func (c *Candidates) IsDelegatorStakeSufficient(address types.Address, pubkey types.Pubkey, coin types.CoinSymbol, amount *big.Int) bool {
	stakes := c.GetStakes(pubkey)
	if len(stakes) < MaxDelegatorsPerCandidate {
		return true
	}

	stakeValue := c.calculateBipValue(coin, amount, true, true)
	for _, stake := range stakes {
		if stakeValue.Cmp(stake.BipValue) == -1 {
			return true
		}
	}

	return false
}

func (c *Candidates) Delegate(address types.Address, pubkey types.Pubkey, coin types.CoinSymbol, value *big.Int) {
	stake := &Stake{
		Owner:          address,
		Coin:           coin,
		Value:          value,
		BipValue:       big.NewInt(0),
		PrevStakeIndex: -1,
		isDirty:        true,
	}

	c.bus.Coins().AddOwnerCandidate(coin, pubkey)

	candidate := c.GetCandidate(pubkey)
	candidate.addUpdate(stake)
}

func (c *Candidates) Edit(pubkey types.Pubkey, rewardAddress types.Address, ownerAddress types.Address) {
	c.loadCandidates()
	c.list[pubkey].setOwner(ownerAddress)
	c.list[pubkey].setReward(rewardAddress)
}

func (c *Candidates) SetOnline(pubkey types.Pubkey) {
	c.loadCandidates()
	c.list[pubkey].setStatus(CandidateStatusOnline)
}

func (c *Candidates) SetOffline(pubkey types.Pubkey) {
	c.loadCandidates()
	c.list[pubkey].setStatus(CandidateStatusOffline)
}

func (c *Candidates) SubStake(address types.Address, pubkey types.Pubkey, coin types.CoinSymbol, value *big.Int) {
	stake := c.GetStakeOfAddress(pubkey, address, coin)
	stake.subValue(value)
}

func (c *Candidates) GetCandidates() []*Candidate {
	c.loadCandidates()
	var candidates []*Candidate
	for _, candidate := range c.list {
		candidates = append(candidates, candidate)
	}

	return candidates
}

func (c *Candidates) GetTotalStake(pubkey types.Pubkey) *big.Int {
	c.loadCandidates()
	candidate := c.list[pubkey]
	if candidate.totalBipStake == nil {
		path := []byte{mainPrefix}
		path = append(path, pubkey[:]...)
		path = append(path, totalStakePrefix)
		_, enc := c.iavl.Get(path)
		if len(enc) == 0 {
			candidate.totalBipStake = big.NewInt(0)
			return big.NewInt(0)
		}

		candidate.totalBipStake = big.NewInt(0).SetBytes(enc)
	}

	return candidate.totalBipStake
}

func (c *Candidates) GetStakes(pubkey types.Pubkey) []*Stake {
	state := c.getStakeState(pubkey)

	var stakes []*Stake
	i := 0
	currentIndex := state.Tail
	for i < state.Count && currentIndex != -1 {
		stake := c.getStakeAtIndex(pubkey, currentIndex)
		stakes = append(stakes, stake)

		currentIndex = stake.PrevStakeIndex
	}

	return stakes
}

func (c *Candidates) StakesCount(pubkey types.Pubkey) int {
	state := c.getStakeState(pubkey)

	return state.Count
}

func (c *Candidates) GetStakeOfAddress(pubkey types.Pubkey, address types.Address, coin types.CoinSymbol) *Stake {
	stakes := c.GetStakes(pubkey)
	for _, stake := range stakes {
		if stake.Owner == address && stake.Coin == coin {
			return stake
		}
	}

	return nil
}

func (c *Candidates) GetStakeValueOfAddress(pubkey types.Pubkey, address types.Address, coin types.CoinSymbol) *big.Int {
	stake := c.GetStakeOfAddress(pubkey, address, coin)
	if stake == nil {
		return nil
	}

	return stake.Value
}

func (c *Candidates) GetCandidateOwner(pubkey types.Pubkey) types.Address {
	c.loadCandidates()

	return c.list[pubkey].OwnerAddress
}

func (c *Candidates) loadCandidates() {
	if c.loaded {
		return
	}

	c.loaded = true

	path := []byte{mainPrefix}
	_, enc := c.iavl.Get(path)
	if len(enc) == 0 {
		c.list = map[types.Pubkey]*Candidate{}
		return
	}

	var candidates []Candidate
	if err := rlp.DecodeBytes(enc, candidates); err != nil {
		panic(fmt.Sprintf("failed to decode candidates: %s", err))
		return
	}

	list := map[types.Pubkey]*Candidate{}
	for _, candidate := range candidates {
		list[candidate.PubKey] = &candidate
	}
}

func (c *Candidates) getStakeState(pubkey types.Pubkey) *stakesState {
	c.loadCandidates()
	candidate := c.list[pubkey]
	if candidate.stakesState == nil {
		path := []byte{mainPrefix}
		path = append(path, pubkey[:]...)
		path = append(path, totalStakePrefix)
		_, enc := c.iavl.Get(path)
		if len(enc) == 0 {
			candidate.stakesState = &stakesState{
				Count:   0,
				Tail:    0,
				isDirty: false,
			}
			return candidate.stakesState
		}

		var stakesState stakesState
		if err := rlp.DecodeBytes(enc, &stakesState); err != nil {
			panic(fmt.Sprintf("failed to decode stakes state: %s", err))
		}

		candidate.stakesState = &stakesState
	}

	return candidate.stakesState
}

func (c *Candidates) getStakeAtIndex(pubkey types.Pubkey, index int) *Stake {
	c.loadCandidates()
	candidate := c.list[pubkey]
	if candidate.stakes[index] == nil {
		path := []byte{mainPrefix}
		path = append(path, pubkey[:]...)
		path = append(path, stakesPrefix)
		path = append(path, []byte(fmt.Sprintf("%d", index))...)
		_, enc := c.iavl.Get(path)
		if len(enc) == 0 {
			return nil
		}

		var stake Stake
		if err := rlp.DecodeBytes(enc, &stake); err != nil {
			panic(fmt.Sprintf("failed to decode stake: %s", err))
		}

		candidate.stakes[index] = &stake
	}

	return candidate.stakes[index]
}

func (c *Candidates) setStakeAtIndex(pubkey types.Pubkey, index int, stake *Stake) {
	c.loadCandidates()
	candidate := c.list[pubkey]
	candidate.stakes[index] = stake
}

func (c *Candidates) calculateBipValue(coinSymbol types.CoinSymbol, amount *big.Int, includeSelf, includeUpdates bool) *big.Int {
	if coinSymbol.IsBaseCoin() {
		return big.NewInt(0).Set(amount)
	}

	totalAmount := big.NewInt(0)
	if includeSelf {
		totalAmount.Set(amount)
	}

	candidates := c.GetCandidates()
	for _, candidate := range candidates {
		stakes := c.GetStakes(candidate.PubKey)
		for _, stake := range stakes {
			if stake.Coin == coinSymbol {
				totalAmount.Add(totalAmount, stake.Value)
			}
		}

		if includeUpdates {
			for _, update := range candidate.updates {
				if update.Coin == coinSymbol {
					totalAmount.Add(totalAmount, update.Value)
				}
			}
		}
	}

	coin := c.bus.Coins().GetCoin(coinSymbol)

	return formula.CalculateSaleReturn(coin.Volume, coin.Reserve, coin.Crr, totalAmount)
}

func (c *Candidates) DeleteCoin(pubkey types.Pubkey, coinSymbol types.CoinSymbol) {

}
