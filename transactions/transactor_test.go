package transactions

import (
	"context"
	"errors"
	"math/big"
	"reflect"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind/backends"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	gethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/contracts/ens/contract"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	gethparams "github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/suite"

	"github.com/status-im/status-go/account"
	"github.com/status-im/status-go/params"
	"github.com/status-im/status-go/rpc"
	"github.com/status-im/status-go/sign"
	"github.com/status-im/status-go/transactions/fake"

	. "github.com/status-im/status-go/t/utils"
)

func TestTxQueueTestSuite(t *testing.T) {
	suite.Run(t, new(TxQueueTestSuite))
}

type TxQueueTestSuite struct {
	suite.Suite
	server            *gethrpc.Server
	client            *gethrpc.Client
	txServiceMockCtrl *gomock.Controller
	txServiceMock     *fake.MockPublicTransactionPoolAPI
	nodeConfig        *params.NodeConfig

	manager *Transactor
}

func (s *TxQueueTestSuite) SetupTest() {
	s.txServiceMockCtrl = gomock.NewController(s.T())

	s.server, s.txServiceMock = fake.NewTestServer(s.txServiceMockCtrl)
	s.client = gethrpc.DialInProc(s.server)
	rpcClient, _ := rpc.NewClient(s.client, params.UpstreamRPCConfig{})
	// expected by simulated backend
	chainID := gethparams.AllEthashProtocolChanges.ChainID.Uint64()
	nodeConfig, err := params.NewNodeConfig("/tmp", "", params.FleetBeta, chainID)
	s.Require().NoError(err)
	s.nodeConfig = nodeConfig

	s.manager = NewTransactor()
	s.manager.sendTxTimeout = time.Second
	s.manager.SetNetworkID(chainID)
	s.manager.SetRPC(rpcClient, time.Second)
}

func (s *TxQueueTestSuite) TearDownTest() {
	s.txServiceMockCtrl.Finish()
	s.server.Stop()
	s.client.Close()
}

var (
	testGas      = hexutil.Uint64(defaultGas + 1)
	testGasPrice = (*hexutil.Big)(big.NewInt(10))
	testNonce    = hexutil.Uint64(10)
)

func (s *TxQueueTestSuite) setupTransactionPoolAPI(args SendTxArgs, returnNonce, resultNonce hexutil.Uint64, account *account.SelectedExtKey, txErr error, signArgs *sign.TxArgs) {
	// Expect calls to gas functions only if there are no user defined values.
	// And also set the expected gas and gas price for RLP encoding the expected tx.
	var usedGas hexutil.Uint64
	var usedGasPrice *big.Int
	s.txServiceMock.EXPECT().GetTransactionCount(gomock.Any(), account.Address, gethrpc.PendingBlockNumber).Return(&returnNonce, nil)
	if signArgs != nil && signArgs.GasPrice != nil {
		usedGasPrice = (*big.Int)(signArgs.GasPrice)
	} else if args.GasPrice == nil {
		usedGasPrice = (*big.Int)(testGasPrice)
		s.txServiceMock.EXPECT().GasPrice(gomock.Any()).Return(testGasPrice, nil)
	} else {
		usedGasPrice = (*big.Int)(args.GasPrice)
	}
	if signArgs != nil && signArgs.Gas != nil {
		usedGas = *signArgs.Gas
	} else if args.Gas == nil {
		s.txServiceMock.EXPECT().EstimateGas(gomock.Any(), gomock.Any()).Return(testGas, nil)
		usedGas = testGas
	} else {
		usedGas = *args.Gas
	}
	// Prepare the transaction anD RLP encode it.
	data := s.rlpEncodeTx(args, s.nodeConfig, account, &resultNonce, usedGas, usedGasPrice)
	// Expect the RLP encoded transaction.
	s.txServiceMock.EXPECT().SendRawTransaction(gomock.Any(), data).Return(gethcommon.Hash{}, txErr)
}

func (s *TxQueueTestSuite) rlpEncodeTx(args SendTxArgs, config *params.NodeConfig, account *account.SelectedExtKey, nonce *hexutil.Uint64, gas hexutil.Uint64, gasPrice *big.Int) hexutil.Bytes {
	newTx := types.NewTransaction(
		uint64(*nonce),
		*args.To,
		args.Value.ToInt(),
		uint64(gas),
		gasPrice,
		[]byte(args.Input),
	)
	chainID := big.NewInt(int64(config.NetworkID))
	signedTx, err := types.SignTx(newTx, types.NewEIP155Signer(chainID), account.AccountKey.PrivateKey)
	s.NoError(err)
	data, err := rlp.EncodeToBytes(signedTx)
	s.NoError(err)
	return hexutil.Bytes(data)
}

func (s *TxQueueTestSuite) TestCompleteTransaction() {
	key, _ := crypto.GenerateKey()
	selectedAccount := &account.SelectedExtKey{
		Address:    account.FromAddress(TestConfig.Account1.Address),
		AccountKey: &keystore.Key{PrivateKey: key},
	}
	testCases := []struct {
		name     string
		gas      *hexutil.Uint64
		gasPrice *hexutil.Big
	}{
		{
			"noGasDef",
			nil,
			nil,
		},
		{
			"gasDefined",
			&testGas,
			nil,
		},
		{
			"gasPriceDefined",
			nil,
			testGasPrice,
		},
		{
			"nilSignTransactionSpecificArgs",
			nil,
			nil,
		},
	}

	for _, testCase := range testCases {
		s.T().Run(testCase.name, func(t *testing.T) {
			s.SetupTest()
			args := SendTxArgs{
				From:     account.FromAddress(TestConfig.Account1.Address),
				To:       account.ToAddress(TestConfig.Account2.Address),
				Gas:      testCase.gas,
				GasPrice: testCase.gasPrice,
			}
			s.setupTransactionPoolAPI(args, testNonce, testNonce, selectedAccount, nil, &sign.TxArgs{})

			result := s.manager.SendTransaction(args, selectedAccount)
			s.NoError(result.Error)
			s.False(reflect.DeepEqual(result.Response.Hash(), gethcommon.Hash{}), "transaction was never queued or completed")
		})
	}
}

func (s *TxQueueTestSuite) TestAccountMismatch() {
	selectedAccount := &account.SelectedExtKey{
		Address: account.FromAddress(TestConfig.Account2.Address),
	}

	args := SendTxArgs{
		From: account.FromAddress(TestConfig.Account1.Address),
		To:   account.ToAddress(TestConfig.Account2.Address),
	}

	result := s.manager.SendTransaction(args, selectedAccount) // nolint: errcheck
	s.EqualError(result.Error, ErrInvalidCompleteTxSender.Error())
}

// TestLocalNonce verifies that local nonce will be used unless
// upstream nonce is updated and higher than a local
// in test we will run 3 transaction with nonce zero returned by upstream
// node, after each call local nonce will be incremented
// then, we return higher nonce, as if another node was used to send 2 transactions
// upstream nonce will be equal to 5, we update our local counter to 5+1
// as the last step, we verify that if tx failed nonce is not updated
func (s *TxQueueTestSuite) TestLocalNonce() {
	txCount := 3
	key, _ := crypto.GenerateKey()
	selectedAccount := &account.SelectedExtKey{
		Address:    account.FromAddress(TestConfig.Account1.Address),
		AccountKey: &keystore.Key{PrivateKey: key},
	}
	nonce := hexutil.Uint64(0)

	for i := 0; i < txCount; i++ {
		args := SendTxArgs{
			From: account.FromAddress(TestConfig.Account1.Address),
			To:   account.ToAddress(TestConfig.Account2.Address),
		}
		s.setupTransactionPoolAPI(args, nonce, hexutil.Uint64(i), selectedAccount, nil, nil)

		result := s.manager.SendTransaction(args, selectedAccount)
		s.NoError(result.Error)
		resultNonce, _ := s.manager.localNonce.Load(args.From)
		s.Equal(uint64(i)+1, resultNonce.(uint64))
	}

	nonce = hexutil.Uint64(5)
	args := SendTxArgs{
		From: account.FromAddress(TestConfig.Account1.Address),
		To:   account.ToAddress(TestConfig.Account2.Address),
	}

	s.setupTransactionPoolAPI(args, nonce, nonce, selectedAccount, nil, nil)

	result := s.manager.SendTransaction(args, selectedAccount)
	s.NoError(result.Error)

	resultNonce, _ := s.manager.localNonce.Load(args.From)
	s.Equal(uint64(nonce)+1, resultNonce.(uint64))

	testErr := errors.New("test")
	s.txServiceMock.EXPECT().GetTransactionCount(gomock.Any(), selectedAccount.Address, gethrpc.PendingBlockNumber).Return(nil, testErr)
	args = SendTxArgs{
		From: account.FromAddress(TestConfig.Account1.Address),
		To:   account.ToAddress(TestConfig.Account2.Address),
	}

	result = s.manager.SendTransaction(args, selectedAccount)
	s.EqualError(testErr, result.Error.Error())
	resultNonce, _ = s.manager.localNonce.Load(args.From)
	s.Equal(uint64(nonce)+1, resultNonce.(uint64))
}

func (s *TxQueueTestSuite) TestContractCreation() {
	key, _ := crypto.GenerateKey()
	testaddr := crypto.PubkeyToAddress(key.PublicKey)
	genesis := core.GenesisAlloc{
		testaddr: {Balance: big.NewInt(100000000000)},
	}
	backend := backends.NewSimulatedBackend(genesis)
	selectedAccount := &account.SelectedExtKey{
		Address:    testaddr,
		AccountKey: &keystore.Key{PrivateKey: key},
	}
	s.manager.sender = backend
	s.manager.gasCalculator = backend
	s.manager.pendingNonceProvider = backend
	tx := SendTxArgs{
		From:  testaddr,
		Input: hexutil.Bytes(gethcommon.FromHex(contract.ENSBin)),
	}

	result := s.manager.SendTransaction(tx, selectedAccount)
	s.NoError(result.Error)
	backend.Commit()
	hash := result.Response.Hash()
	receipt, err := backend.TransactionReceipt(context.TODO(), hash)
	s.NoError(err)
	s.Equal(crypto.CreateAddress(testaddr, 0), receipt.ContractAddress)
}
