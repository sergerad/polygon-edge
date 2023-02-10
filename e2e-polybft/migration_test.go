package e2e

import (
	"context"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"github.com/0xPolygon/polygon-edge/crypto"
	"github.com/0xPolygon/polygon-edge/e2e/framework"
	itrie "github.com/0xPolygon/polygon-edge/state/immutable-trie"
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/stretchr/testify/assert"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/umbracle/ethgo"
)

func TestName(t *testing.T) {
	userKey, _ := crypto.GenerateECDSAKey()
	userAddr := crypto.PubKeyToAddress(&userKey.PublicKey)
	userAddrEthgo := ethgo.Address(userAddr)
	userKey2, _ := crypto.GenerateECDSAKey()
	userAddr2 := crypto.PubKeyToAddress(&userKey2.PublicKey)
	userAddrEthgo2 := ethgo.Address(userAddr2)

	srvs := framework.NewTestServers(t, 1, func(config *framework.TestServerConfig) {
		config.SetConsensus(framework.ConsensusDev)
		config.Premine(userAddr, ethgo.Ether(10))
	})
	srv := srvs[0]

	rpcClient := srv.JSONRPC()

	// Fetch the balances before sending
	balanceSender, err := rpcClient.Eth().GetBalance(
		userAddrEthgo,
		ethgo.Latest,
	)
	assert.NoError(t, err)
	t.Log(balanceSender)

	balanceReceiver, err := rpcClient.Eth().GetBalance(
		userAddrEthgo2,
		ethgo.Latest,
	)
	assert.NoError(t, err)
	t.Log(balanceReceiver)

	// Set the preSend balances
	previousSenderBalance := balanceSender
	previousReceiverBalance := balanceReceiver

	block, err := rpcClient.Eth().GetBlockByNumber(ethgo.Latest, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(block.Number, block.Hash.String(), block.StateRoot.String())

	// Do the transfer
	ctx, cancel := context.WithTimeout(context.Background(), framework.DefaultTimeout)
	defer cancel()

	txn := &framework.PreparedTransaction{
		From:     userAddr,
		To:       &userAddr2,
		GasPrice: big.NewInt(1048576),
		Gas:      1000000,
		Value:    ethgo.Gwei(10000),
	}

	receipt, err := srv.SendRawTx(ctx, txn, userKey)
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
	defer db2.Close()

	stateStorage := itrie.NewKV(db)
	stateStorageNew := itrie.NewKV(db2)

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

	rootNode, _, err := itrie.GetNode(stateRoot.Bytes(), stateStorage)
	if err != nil {
		t.Fatal()
	}

	err = itrie.CopyTrie1(stateRoot.Bytes(), stateStorage, stateStorageNew, nil)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("copy")
	it = db2.NewIterator(nil, nil)
	id = 0
	for {
		v := it.Next()
		if v == false {
			break
		}
		t.Log(id, it.Key(), it.Value())
		id++
	}

	newTrie := itrie.NewTrieWithRoot(rootNode)
	newStateRoot, err := newTrie.Txn(stateStorageNew).Hash()
	if err != nil {
		t.Fatal(err)
	}

	stateRoot3, err := itrie.HashChecker1(rootNode, stateStorageNew)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(types.BytesToHash(newStateRoot).String())
	t.Log(stateRoot.String())
	t.Log(stateRoot3.String())
	t.Fatal()

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
