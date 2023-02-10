package e2e

import (
	"github.com/0xPolygon/polygon-edge/consensus/polybft/contractsapi"
	"github.com/0xPolygon/polygon-edge/crypto"
	frameworkpolybft "github.com/0xPolygon/polygon-edge/e2e-polybft/framework"
	"github.com/0xPolygon/polygon-edge/e2e/framework"
	"golang.org/x/crypto/sha3"

	itrie "github.com/0xPolygon/polygon-edge/state/immutable-trie"
	"github.com/0xPolygon/polygon-edge/txrelayer"
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/umbracle/ethgo"
	"github.com/umbracle/ethgo/wallet"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMigration(t *testing.T) {
	os.Setenv("EDGE_BINARY", "/Users/boris/GolandProjects/polygon-edge/polygon-edge")
	os.Setenv("E2E_TESTS", "true")
	os.Setenv("E2E_LOGS", "true")

	userKey, _ := wallet.GenerateKey()
	userAddr := userKey.Address()
	userKey2, _ := wallet.GenerateKey()
	userAddr2 := userKey2.Address()

	srvs := framework.NewTestServers(t, 1, func(config *framework.TestServerConfig) {
		config.SetConsensus(framework.ConsensusDev)
		config.Premine(types.Address(userAddr), ethgo.Ether(10))
	})
	srv := srvs[0]

	rpcClient := srv.JSONRPC()

	// Fetch the balances before sending
	balanceSender, err := rpcClient.Eth().GetBalance(
		userAddr,
		ethgo.Latest,
	)
	assert.NoError(t, err)

	balanceReceiver, err := rpcClient.Eth().GetBalance(
		userAddr2,
		ethgo.Latest,
	)
	assert.NoError(t, err)

	// Set the preSend balances
	previousSenderBalance := balanceSender
	previousReceiverBalance := balanceReceiver

	block, err := rpcClient.Eth().GetBlockByNumber(ethgo.Latest, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(block.Number, block.Hash.String(), block.StateRoot.String())

	relayer, err := txrelayer.NewTxRelayer(txrelayer.WithIPAddress(srv.HTTPJSONRPCURL()))
	require.NoError(t, err)

	receipt, err := relayer.SendTransaction(&ethgo.Transaction{
		From:     userAddr,
		To:       &userAddr2,
		GasPrice: 1048576,
		Gas:      1000000,
		Value:    ethgo.Gwei(10000),
	}, userKey)
	assert.NoError(t, err)
	assert.NotNil(t, receipt)

	receipt, err = relayer.SendTransaction(&ethgo.Transaction{
		From:     userAddr,
		GasPrice: 1048576,
		Gas:      1000000,
		Input:    contractsapi.TestWriteBlockMetadata.Bytecode,
	}, userKey)
	deployedContractBalance := receipt.ContractAddress
	assert.NoError(t, err)
	assert.NotNil(t, receipt)

	// Fetch the balances after sending
	balanceSender, err = rpcClient.Eth().GetBalance(
		ethgo.Address(userAddr),
		ethgo.Latest,
	)
	assert.NoError(t, err)

	balanceReceiver, err = rpcClient.Eth().GetBalance(
		ethgo.Address(userAddr2),
		ethgo.Latest,
	)
	assert.NoError(t, err)

	t.Log(previousSenderBalance, balanceSender)
	t.Log(previousReceiverBalance, balanceReceiver)

	initReceipt, err := ABITransaction(relayer, userKey, contractsapi.TestWriteBlockMetadata, receipt.ContractAddress, "init")
	if err != nil {
		t.Fatal(err)
	}

	t.Log(initReceipt.Status)

	block, err = rpcClient.Eth().GetBlockByNumber(ethgo.Latest, true)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(block.Number)
	stateRoot := block.StateRoot

	path := srvs[0].Config.RootDir
	srvs[0].Stop()
	dbOLD := "trie"
	dbNEW := "trieNew"

	time.Sleep(time.Second)
	db, err := leveldb.OpenFile(filepath.Join(path, dbOLD), &opt.Options{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	newTrieDB := filepath.Join(path, dbNEW)
	os.RemoveAll(newTrieDB)
	db2, err := leveldb.OpenFile(newTrieDB, nil)
	if err != nil {
		t.Fatal(err)
	}

	stateStorage := itrie.NewKV(db)
	stateStorageNew := itrie.NewKV(db2)

	exSnapshot, err := itrie.NewState(stateStorage).NewSnapshotAt(types.Hash(stateRoot))
	if err != nil {
		t.Fatal(err)
	}
	t.Log("get addr")
	t.Log(exSnapshot.GetAccount(types.Address(userAddr)))

	rootNode, _, err := itrie.GetNode(stateRoot.Bytes(), stateStorage)
	if err != nil {
		t.Fatal()
	}
	oldTrie := itrie.NewTrieWithRoot(rootNode)
	t.Log("Get old trie")
	t.Log(oldTrie.Get(crypto.Keccak256(userAddr.Bytes()), stateStorage))
	t.Log(oldTrie.Get(crypto.Keccak256(userAddr2.Bytes()), stateStorage))

	err = itrie.CopyTrie1(stateRoot.Bytes(), stateStorage, stateStorageNew, nil)
	if err != nil {
		t.Fatal(err)
	}

	newTrie := itrie.NewTrieWithRoot(rootNode)
	newStateRoot, err := newTrie.Txn(stateStorageNew).Hash()
	if err != nil {
		t.Fatal(err)
	}

	t.Log("Get new trie")
	t.Log(newTrie.Get(crypto.Keccak256(userAddr.Bytes()), stateStorageNew))
	t.Log(newTrie.Get(crypto.Keccak256(userAddr2.Bytes()), stateStorageNew))

	stateRoot3, err := itrie.HashChecker1(stateRoot.Bytes(), stateStorageNew)
	if err != nil {
		t.Fatal(err)
	}
	err = db2.Close()
	if err != nil {
		t.Fatal(err)
	}

	t.Log(types.BytesToHash(newStateRoot).String())
	t.Log(stateRoot.String())
	t.Log(stateRoot3.String())

	cluster := frameworkpolybft.NewTestCluster(t, 7,
		frameworkpolybft.WithNonValidators(2),
		frameworkpolybft.WithValidatorSnapshot(5),
		frameworkpolybft.WithGenesisState(newTrieDB, types.Hash(stateRoot)),
	)
	defer cluster.Stop()

	require.NoError(t, cluster.WaitForBlock(5, 1*time.Minute))

	senderBalanceAfterMigration, err := cluster.Servers[0].JSONRPC().Eth().GetBalance(userAddr, ethgo.Latest)
	if err != nil {
		t.Fatal(err)
	}
	receiverBalanceAfterMigration, err := cluster.Servers[0].JSONRPC().Eth().GetBalance(userAddr2, ethgo.Latest)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(senderBalanceAfterMigration, receiverBalanceAfterMigration)
	t.Log(balanceSender, balanceReceiver)

	require.NoError(t, cluster.WaitForBlock(10, 1*time.Minute))

	deployedCode, err := cluster.Servers[0].JSONRPC().Eth().GetCode(deployedContractBalance, ethgo.Latest)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(deployedCode)
	t.Log(*types.EncodeBytes(contractsapi.TestWriteBlockMetadata.DeployedBytecode))
	require.Equal(t, deployedCode, *types.EncodeBytes(contractsapi.TestWriteBlockMetadata.DeployedBytecode))
}

func PrintDB(db *leveldb.DB, t *testing.T) {
	t.Log("copy")
	it := db.NewIterator(nil, nil)
	id := 0
	for {
		v := it.Next()
		if v == false {
			break
		}
		t.Log(id, it.Key(), it.Value())
		id++
	}
}

/*
	//000001.log      CURRENT         LOCK            LOG             MANIFEST-000000
	files := []string{"000001.log", "CURRENT", "LOCK", "LOG", "MANIFEST-000000"}

	for _, file := range files {
		fData, err := ioutil.ReadFile(filepath.Join(path, dbNEW, file))
		if err != nil {
			t.Fatal(err)
		}
		t.Log(file, types.BytesToHash(hashit(fData)).String())
	}


*/

func hashit(k []byte) []byte {
	h := sha3.NewLegacyKeccak256()
	h.Write(k)

	return h.Sum(nil)
}
