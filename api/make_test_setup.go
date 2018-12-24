package api

import (
	"crypto/ecdsa"
	"fmt"
	"github.com/MinterTeam/minter-go-node/core/transaction"
	"github.com/MinterTeam/minter-go-node/core/types"
	"github.com/MinterTeam/minter-go-node/crypto"
	"github.com/MinterTeam/minter-go-node/helpers"
	"github.com/MinterTeam/minter-go-node/rlp"
	"github.com/miguelmota/go-ethereum-hdwallet"
	"github.com/pkg/errors"
	"math/big"
	"math/rand"
)

type TestSetupResponse struct {
	Mnemonic   string           `json:"mnemonic"`
	Address    types.Address    `json:"address"`
	CoinSymbol types.CoinSymbol `json:"coin_symbol"`
}

var letterRunes = []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890")

func MakeTestSetup(env string) (*TestSetupResponse, error) {
	mnemonic := "tag volcano eight thank tide danger coast health above argue embrace heavy"
	wallet, err := hdwallet.NewFromMnemonic(mnemonic)
	if err != nil {
		return nil, err
	}

	path := hdwallet.MustParseDerivationPath("m/44'/60'/0'/0/0")
	account, err := wallet.Derive(path, false)
	if err != nil {
		return nil, err
	}

	fmt.Println(account.Address.Hex()) // 0xC49926C4124cEe1cbA0Ea94Ea31a6c12318df947

	if env != "bot" {
		return nil, errors.New("Unknown env")
	}

	pkey, err := crypto.GenerateKey()
	if err != nil {
		return nil, err
	}

	address := crypto.PubkeyToAddress(pkey.PublicKey)
	state := blockchain.GetDeliverState()

	// add 100,000 MNT to balance
	state.AddBalance(address, types.GetBaseCoin(), helpers.BipToPip(big.NewInt(100000)))

	volume := helpers.BipToPip(big.NewInt(10000))
	reserve := helpers.BipToPip(big.NewInt(10000))
	var coinSymbol types.CoinSymbol
	for i := range coinSymbol {
		coinSymbol[i] = byte(letterRunes[rand.Intn(len(letterRunes))])
	}

	// create coin with random symbol
	state.CreateCoin(coinSymbol, "TEST COIN", volume, 10, reserve)
	state.AddBalance(address, coinSymbol, volume)

	// create candidate
	pubkey := make([]byte, 32)
	rand.Read(pubkey)
	state.CreateCandidate(types.Address{}, types.Address{}, pubkey, 10, 0, types.GetBaseCoin(), helpers.BipToPip(big.NewInt(1)))

	// update state
	blockchain.WaitCommit()

	err = sendTx(pkey)
	if err != nil {
		return nil, err
	}

	err = delegateTx(pkey, pubkey)
	if err != nil {
		return nil, err
	}

	return &TestSetupResponse{
		Mnemonic:   mnemonic,
		Address:    address,
		CoinSymbol: coinSymbol,
	}, nil
}

func sendTx(pkey *ecdsa.PrivateKey) error {
	value := helpers.BipToPip(big.NewInt(10))
	to := types.Address([20]byte{1})

	data := transaction.SendData{
		Coin:  types.GetBaseCoin(),
		To:    to,
		Value: value,
	}

	encodedData, err := rlp.EncodeToBytes(data)

	if err != nil {
		return err
	}

	tx := transaction.Transaction{
		Nonce:         1,
		GasPrice:      big.NewInt(1),
		GasCoin:       types.GetBaseCoin(),
		Type:          transaction.TypeSend,
		Data:          encodedData,
		SignatureType: transaction.SigTypeSingle,
	}

	if err := tx.Sign(pkey); err != nil {
		return err
	}

	encodedTx, err := rlp.EncodeToBytes(tx)
	if err != nil {
		return err
	}

	_, err = client.BroadcastTxCommit(encodedTx)
	if err != nil {
		return err
	}

	return nil
}

func delegateTx(pkey *ecdsa.PrivateKey, candidatePubKey types.Pubkey) error {
	value := helpers.BipToPip(big.NewInt(100))

	data := transaction.DelegateData{
		PubKey: candidatePubKey,
		Coin:   types.GetBaseCoin(),
		Stake:  value,
	}

	encodedData, err := rlp.EncodeToBytes(data)
	if err != nil {
		return err
	}

	tx := transaction.Transaction{
		Nonce:         2,
		GasPrice:      big.NewInt(1),
		GasCoin:       types.GetBaseCoin(),
		Type:          transaction.TypeDelegate,
		Data:          encodedData,
		SignatureType: transaction.SigTypeSingle,
	}

	if err := tx.Sign(pkey); err != nil {
		return err
	}

	encodedTx, err := rlp.EncodeToBytes(tx)
	if err != nil {
		return err
	}

	_, err = client.BroadcastTxCommit(encodedTx)
	if err != nil {
		return err
	}

	return nil
}
