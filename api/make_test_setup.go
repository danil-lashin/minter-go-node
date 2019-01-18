package api

import (
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/MinterTeam/minter-go-node/core/transaction"
	"github.com/MinterTeam/minter-go-node/core/types"
	"github.com/MinterTeam/minter-go-node/crypto"
	"github.com/MinterTeam/minter-go-node/helpers"
	"github.com/MinterTeam/minter-go-node/log"
	"github.com/MinterTeam/minter-go-node/rlp"
	"github.com/Swipecoin/go-bip44"
	"github.com/miguelmota/go-ethereum-hdwallet"
	"math/big"
	"math/rand"
	"time"
)

type TestSetupResponse struct {
	Mnemonic   string `json:"mnemonic"`
	Address    string `json:"address"`
	CoinSymbol string `json:"coin_symbol"`
	Candidate  string `json:"candidate"`
}

func MakeTestSetup(env string) (*TestSetupResponse, error) {
	nonce := uint64(1)
	log.Error("T1")
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
	log.Error("T2")
	// add 100,000 MNT to balance
	state.AddBalance(address, types.GetBaseCoin(), helpers.BipToPip(big.NewInt(1000000)))

	var coinSymbol types.CoinSymbol
	copy(coinSymbol[:], []byte("TESTBOT"))

	if !state.CoinExists(coinSymbol) {
		log.Error("T3")
		blockchain.WaitCommit()
		err = createCoin(pkey, coinSymbol, nonce)
		if err != nil {
			return nil, err
		}
		nonce++
	}
	log.Error("T4")

	value := helpers.BipToPip(big.NewInt(1000))
	state.AddBalance(address, coinSymbol, value)
	state.AddCoinVolume(coinSymbol, value)
	state.AddCoinReserve(coinSymbol, value)

	blockchain.WaitCommit()

	log.Error("T5")
	// create candidate
	pubkey := make([]byte, 32)
	rand.Seed(time.Now().Unix())
	rand.Read(pubkey)
	err = createCandidate(pkey, pubkey, nonce)
	if err != nil {
		return nil, err
	}
	nonce++

	log.Error("T6")

	for i := 0; i < 4; i++ {
		err = sendTx(pkey, nonce)
		if err != nil {
			return nil, err
		}
		nonce++
	}

	log.Error("T7")
	b, _ := hex.DecodeString("b52951425d2517504f767215ca77a9be3e0cd788fd72443da9b174fc686a37f0")
	err = delegateTx(pkey, b, nonce)
	if err != nil {
		return nil, err
	}

	log.Error("T8")

	return &TestSetupResponse{
		Mnemonic:   mnemonic.Value,
		Address:    address.String(),
		CoinSymbol: coinSymbol.String(),
		Candidate:  fmt.Sprintf("Mp%x", pubkey),
	}, nil
}

func createCoin(pkey *ecdsa.PrivateKey, coinSymbol types.CoinSymbol, nonce uint64) error {
	data := transaction.CreateCoinData{
		Name:                 "",
		Symbol:               coinSymbol,
		InitialAmount:        helpers.BipToPip(big.NewInt(100)),
		InitialReserve:       helpers.BipToPip(big.NewInt(100)),
		ConstantReserveRatio: 100,
	}

	encodedData, err := rlp.EncodeToBytes(data)

	if err != nil {
		return err
	}

	tx := transaction.Transaction{
		Nonce:         nonce,
		GasPrice:      big.NewInt(1),
		GasCoin:       types.GetBaseCoin(),
		Type:          transaction.TypeCreateCoin,
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

	result, err := client.BroadcastTxCommit(encodedTx)
	if err != nil {
		return err
	}

	if result.CheckTx.Code != 0 {
		return errors.New(fmt.Sprintf("Error %d: %s", result.CheckTx.Code, result.CheckTx.Log))
	}

	return nil
}

func createCandidate(pkey *ecdsa.PrivateKey, publicKey types.Pubkey, nonce uint64) error {
	data := transaction.DeclareCandidacyData{
		Address:    crypto.PubkeyToAddress(pkey.PublicKey),
		PubKey:     publicKey,
		Commission: 10,
		Coin:       types.GetBaseCoin(),
		Stake:      helpers.BipToPip(big.NewInt(1000)),
	}

	encodedData, err := rlp.EncodeToBytes(data)

	if err != nil {
		return err
	}

	tx := transaction.Transaction{
		Nonce:         nonce,
		GasPrice:      big.NewInt(1),
		GasCoin:       types.GetBaseCoin(),
		Type:          transaction.TypeDeclareCandidacy,
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

	result, err := client.BroadcastTxCommit(encodedTx)
	if err != nil {
		return err
	}

	if result.CheckTx.Code != 0 {
		return errors.New(fmt.Sprintf("Error %d: %s", result.CheckTx.Code, result.CheckTx.Log))
	}

	return nil
}

func sendTx(pkey *ecdsa.PrivateKey, nonce uint64) error {
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
		Nonce:         nonce,
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

	result, err := client.BroadcastTxCommit(encodedTx)
	if err != nil {
		return err
	}

	if result.CheckTx.Code != 0 {
		return errors.New(fmt.Sprintf("Error %d: %s", result.CheckTx.Code, result.CheckTx.Log))
	}

	return nil
}

func delegateTx(pkey *ecdsa.PrivateKey, candidatePubKey types.Pubkey, nonce uint64) error {
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
		Nonce:         nonce,
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

	result, err := client.BroadcastTxCommit(encodedTx)
	if err != nil {
		return err
	}

	if result.CheckTx.Code != 0 {
		return errors.New(fmt.Sprintf("Error %d: %s", result.CheckTx.Code, result.CheckTx.Log))
	}

	return nil
}
