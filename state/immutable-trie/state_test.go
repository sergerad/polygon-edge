package itrie

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/0xPolygon/polygon-edge/types"
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
	path := "../../test-chain-1"
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

	newState := NewState(stateStorageNew)
	newTrie := newState.newTrie()
	newTrie.root = rootNode
	newState.AddState(stateRoot, newTrie)

	snap2, err := newState.NewSnapshotAt(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	_, netStateRoot := snap2.Commit(nil)

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
	t.Log(acc.String())

	// This is not working
	acc, err = snap2.GetAccount(types.StringToAddress("0x6FdA56C57B0Acadb96Ed5624aC500C0429d59429"))
	if err != nil {
		t.Fatal(err)
	}
	t.Log(acc.String())

	//000001.log      CURRENT         LOCK            LOG             MANIFEST-000000
	files := []string{"000001.log", "CURRENT", "LOCK", "LOG", "MANIFEST-000000"}
	for _, file := range files {
		fData, err := ioutil.ReadFile(filepath.Join(path, dbNEW, file))
		if err != nil {
			t.Fatal(err)
		}
		t.Log(file, types.BytesToHash(hashit(fData)).String())
	}

	_, root := snap.Commit(nil)
	t.Log("state roots")
	t.Log(types.BytesToHash(root).String())
	t.Log(types.BytesToHash(netStateRoot).String())
	// sn := &Snapshot{state: state, trie: trie}
	// _ = sn

	t.Log("keys")
	it := db.NewIterator(nil, nil)
	it2 := db.NewIterator(nil, nil)
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
	state_test.go:72: 000001.log 0x525bfb10b7b757008c1cd7e203947d3ab147bd0553caec07ed9d8fd84c666f41
   state_test.go:72: CURRENT 0xa9fab754f1d15003108a400d3a46cb94cbae5407a86aac699db0041c76177c72
   state_test.go:72: LOCK 0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470
   state_test.go:72: LOG 0x480391049595c8d1b204992f62f86bfc458e41b4f6f9a24bcb581b9a4ad9a730
   state_test.go:72: MANIFEST-000000 0xce560a9b6381d82394c3410f40a657384a30754a3a1dae9a1b5c3549bd91c331

    state_test.go:72: 000001.log 0x525bfb10b7b757008c1cd7e203947d3ab147bd0553caec07ed9d8fd84c666f41
    state_test.go:72: CURRENT 0xa9fab754f1d15003108a400d3a46cb94cbae5407a86aac699db0041c76177c72
    state_test.go:72: LOCK 0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470
    state_test.go:72: LOG 0x09c38d83939ca24d68afa472a4021d25c4e83985af57c33bf40fc84fca23a1ed
    state_test.go:72: MANIFEST-000000 0xce560a9b6381d82394c3410f40a657384a30754a3a1dae9a1b5c3549bd91c331

    state_test.go:74: 000001.log 0x525bfb10b7b757008c1cd7e203947d3ab147bd0553caec07ed9d8fd84c666f41
    state_test.go:74: CURRENT 0xa9fab754f1d15003108a400d3a46cb94cbae5407a86aac699db0041c76177c72
    state_test.go:74: LOCK 0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470
    state_test.go:74: LOG 0x1232db1a8ebd50aba9289934652b6020ef523dc3e68cfc0748310176716edd64
    state_test.go:74: MANIFEST-000000 0xce560a9b6381d82394c3410f40a657384a30754a3a1dae9a1b5c3549bd91c331

    state_test.go:74: 000001.log 0x525bfb10b7b757008c1cd7e203947d3ab147bd0553caec07ed9d8fd84c666f41
    state_test.go:74: CURRENT 0xa9fab754f1d15003108a400d3a46cb94cbae5407a86aac699db0041c76177c72
    state_test.go:74: LOCK 0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470
    state_test.go:74: LOG 0x831a2054bcdd595ff412d992752086de8f905f0c3486f9c4c0d5c952d9623300
    state_test.go:74: MANIFEST-000000 0xce560a9b6381d82394c3410f40a657384a30754a3a1dae9a1b5c3549bd91c331
*/
