package api

import (
	"crypto/ecdsa"
	"errors"
	"github.com/MinterTeam/minter-go-node/core/transaction"
	"github.com/MinterTeam/minter-go-node/core/types"
	"github.com/MinterTeam/minter-go-node/crypto"
	"github.com/MinterTeam/minter-go-node/helpers"
	"github.com/MinterTeam/minter-go-node/rlp"
	"github.com/Swipecoin/go-bip44"
	"github.com/miguelmota/go-ethereum-hdwallet"
	"math/big"
	"math/rand"
)

type TestSetupResponse struct {
	Mnemonic   string           `json:"mnemonic"`
	Address    types.Address    `json:"address"`
	CoinSymbol types.CoinSymbol `json:"coin_symbol"`
	Candidate  types.Pubkey     `json:"candidate"`
}

var letterRunes = []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890")

func MakeTestSetup(env string) (*TestSetupResponse, error) {
	if env != "bot" {
		return nil, errors.New("unknown env")
	}

	bitSize := 128
	mnemonic, _ := bip44.NewMnemonic(bitSize)

	wallet, err := hdwallet.NewFromMnemonic(mnemonic.Value)
	if err != nil {
		return nil, err
	}

	path := hdwallet.MustParseDerivationPath("m/44'/60'/0'/0/0")
	account, err := wallet.Derive(path, false)
	if err != nil {
		return nil, err
	}

	pkeyBytes, _ := wallet.PrivateKeyBytes(account)
	pkey := crypto.ToECDSAUnsafe(pkeyBytes)

	address := crypto.PubkeyToAddress(pkey.PublicKey)
	state := blockchain.GetDeliverState()

	// add 100,000 MNT to balance
	state.AddBalance(address, types.GetBaseCoin(), helpers.BipToPip(big.NewInt(100000)))

	var coinSymbol types.CoinSymbol
	copy(coinSymbol[:], []byte("TESTBOT"))
	state.AddBalance(address, coinSymbol, helpers.BipToPip(big.NewInt(1000)))

	// create candidate
	pubkey := make([]byte, 32)
	rand.Read(pubkey)
	state.CreateCandidate(address, address, pubkey, 10, 0, types.GetBaseCoin(), helpers.BipToPip(big.NewInt(10000)))

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
		Mnemonic:   mnemonic.Value,
		Address:    address,
		CoinSymbol: coinSymbol,
		Candidate:  pubkey,
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
