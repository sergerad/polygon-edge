package itrie

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/0xPolygon/polygon-edge/types"
	"github.com/stretchr/testify/assert"
	"github.com/syndtr/goleveldb/leveldb"

	"github.com/0xPolygon/polygon-edge/state"
)

func TestState(t *testing.T) {
	state.TestState(t, buildPreState)
}

func buildPreState(pre state.PreStates) state.Snapshot {
	storage := NewMemoryStorage()
	st := NewState(storage)
	snap := st.NewSnapshot()

	return snap
}

func TestName(t *testing.T) {
	path := "./testdata"
	dbOLD := "trie"
	dbNEW := "trieNew"
	stateRoot := types.StringToHash("0xc6a643ef265b08f17e555e221dd77b1a8822d96097fb987914db168b34c93cfb")

	db, err := leveldb.OpenFile(filepath.Join(path, dbOLD), nil)
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

	stateStorage := &KVStorage{db}
	stateStorageNew := &KVStorage{db2}

	state := NewState(stateStorage)
	snap, err := state.NewSnapshotAt(stateRoot)
	if err != nil {
		t.Fatal(err)
	}

	rootNode, _, err := GetNode(stateRoot.Bytes(), stateStorage)
	if err != nil {
		t.Fatal()
	}

	newTrie := NewTrie()
	newTrie.root = rootNode
	newState := NewState(stateStorageNew)
	snap2 := &Snapshot{state: newState, trie: newTrie}
	_, newStateRoot := snap2.Commit(nil)

	assert.Equal(t, stateRoot, types.BytesToHash(newStateRoot))

	// This is not working
	acc, err := snap.GetAccount(types.StringToAddress("0xa0d070F081e6A6c135Fdd7778533d97E59627676"))
	if err != nil {
		t.Fatal(err)
	}
	t.Log(acc.String())

	acc, err = snap2.GetAccount(types.StringToAddress("0xa0d070F081e6A6c135Fdd7778533d97E59627676"))
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNil(t, acc)

	// This is not working
	acc, err = snap2.GetAccount(types.StringToAddress("0x6FdA56C57B0Acadb96Ed5624aC500C0429d59429"))
	if err != nil {
		t.Fatal(err)
	}
	assert.NotNil(t, acc)

	_, root := snap.Commit(nil)
	t.Log("state roots")
	assert.Equal(t, newStateRoot, root)
	t.Log(types.BytesToHash(root).String())
	t.Log(types.BytesToHash(newStateRoot).String())
	// sn := &Snapshot{state: state, trie: trie}
	// _ = sn

	t.Log("keys")
	it := db.NewIterator(nil, nil)
	it2 := db2.NewIterator(nil, nil)
	for {
		v := it.Next()
		v2 := it2.Next()
		if v == true && v2 == false {
			t.Fatal()
		}
		if v == false && v2 == false {
			break
		}
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