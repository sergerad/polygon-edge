package itrie

import (
	"fmt"
	"github.com/0xPolygon/polygon-edge/helper/hex"
	"github.com/umbracle/fastrlp"
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

/*
consensus_test.go:117:
test-cluster.go:141: logs enabled for e2e test: ../e2e-logs-1675861635194/TestE2E_Consensus_RegisterValidator
consensus_test.go:156: Block Number=10
consensus_test.go:166: newValidatorAcc 0x9860734dC46250E199d8ed987b2AC0b09862bCE3
consensus_test.go:190: New validator stake=100000000000000000000000
consensus_test.go:204: 10 0x0bce1d24ca94223d6098d7aaccf6157d38fd8e272754340420d5560a7eece430 0x9657fdae5f59fa00e799fd1190858ab433faddd068c7c6cc8bad0497cf067230
consensus_test.go:205: 10
consensus_test.go:206:
*/
func TestName(t *testing.T) {
	path := "/tmp/e2e-polybft-974586914/test-chain-2/"
	dbOLD := "trie"
	dbNEW := "trieNew"
	stateRoot := types.StringToHash("0x9657fdae5f59fa00e799fd1190858ab433faddd068c7c6cc8bad0497cf067230")

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

	rootNode, _, err := GetNode(stateRoot.Bytes(), stateStorage)
	if err != nil {
		t.Fatal()
	}

	err = CopyTrie1(stateRoot.Bytes(), stateStorage, stateStorageNew, nil)
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

	newTrie := NewTrie()
	newTrie.root = rootNode
	//newState := NewState(stateStorageNew)
	newStateRoot, err := newTrie.Txn(stateStorageNew).Hash()
	if err != nil {
		t.Fatal(err)
	}
	h, ok := hasherPool.Get().(*hasher)
	if !ok {
		t.Fatal(ok)
	}
	arena, _ := h.AcquireArena()
	val, err := HashChecker(rootNode, h, arena, 0, stateStorageNew)
	if err != nil {
		t.Error(err)
	}

	h.ReleaseArenas(0)
	hasherPool.Put(h)
	t.Log(types.BytesToHash(newStateRoot).String())
	t.Log(stateRoot.String())
	t.Log(types.BytesToHash(val.Raw()).String())
	t.Fatal()
}

func getCustomNode(hash []byte, storage Storage) (Node, []byte, error) {
	data, ok := storage.Get(hash)
	if !ok {
		return nil, nil, nil
	}

	// NOTE. We dont need to make copies of the bytes because the nodes
	// take the reference from data itself which is a safe copy.
	p := parserPool.Get()
	defer parserPool.Put(p)

	v, err := p.Parse(data)
	if err != nil {
		return nil, nil, err
	}

	if v.Type() != fastrlp.TypeArray {
		return nil, nil, fmt.Errorf("storage item should be an array")
	}

	n, err := decodeNode(v, storage)

	return n, data, err
}

func CopyTrie1(nodeHash []byte, storage Storage, newStorage Storage, agg []byte) error {
	node, data, err := getCustomNode(nodeHash, storage)
	if err != nil {
		return err
	}
	newStorage.Put(nodeHash, data)
	return CopyTrie(node, storage, newStorage, agg)
}

func CopyTrie(node Node, storage Storage, newStorage Storage, agg []byte) error {
	switch n := node.(type) {
	case nil:
		return nil
	case *FullNode:
		for i := range n.children {
			if n.children[i] == nil {
				continue
			}
			err := CopyTrie(n.children[i], storage, newStorage, append(agg, uint8(i)))
			if err != nil {
				return err
			}
		}

	case *ValueNode:
		if n.hash {
			return CopyTrie1(n.buf, storage, newStorage, agg)
		}
		var account state.Account
		if err := account.UnmarshalRlp(n.buf); err != nil {
			fmt.Println("cant parse", err, hex.EncodeToString(encodeCompact(agg)))
		} else {
			code, ok := storage.GetCode(types.BytesToHash(account.CodeHash))
			if !ok {
				fmt.Println("------------------------Code is empty-------------")
			}
			newStorage.SetCode(types.BytesToHash(account.CodeHash), code)

			err = CopyTrie1(account.Root[:], storage, newStorage, nil)
			return err
		}

	case *ShortNode:
		err := CopyTrie(n.child, storage, newStorage, append(agg, n.key...))
		if err != nil {
			return err
		}
	}
	return nil
}

func HashChecker(node Node, h *hasher, a *fastrlp.Arena, d int, storage Storage) (*fastrlp.Value, error) {
	var val *fastrlp.Value

	var aa *fastrlp.Arena

	var idx int

	switch n := node.(type) {
	case *ValueNode:
		if n.hash {
			nd, _, err := GetNode(n.buf, storage)
			if err != nil {
				return nil, err
			}
			return HashChecker(nd, h, a, d, storage)
		}
		return a.NewCopyBytes(n.buf), nil

	case *ShortNode:
		fmt.Println("short")
		child, err := HashChecker(n.child, h, a, d+1, storage)
		if err != nil {
			return nil, err
		}

		val = a.NewArray()
		val.Set(a.NewBytes(encodeCompact(n.key)))
		val.Set(child)

	case *FullNode:
		fmt.Println("full")

		val = a.NewArray()

		aa, idx = h.AcquireArena()

		for _, i := range n.children {
			if i == nil {
				val.Set(a.NewNull())
			} else {
				v, err := HashChecker(i, h, aa, d+1, storage)
				if err != nil {
					return nil, err
				}
				val.Set(v)
			}
		}

		// Add the value
		if n.value == nil {
			val.Set(a.NewNull())
		} else {
			v, err := HashChecker(n.value, h, a, d+1, storage)
			if err != nil {
				return nil, err
			}
			val.Set(v)
		}

	default:
		panic(fmt.Sprintf("unknown node type %v", n))
	}

	if val.Len() < 32 {
		return val, nil
	}

	// marshal RLP value
	h.buf = val.MarshalTo(h.buf[:0])

	if aa != nil {
		h.ReleaseArenas(idx)
	}

	tmp := h.Hash(h.buf)
	hh := node.SetHash(tmp)

	return a.NewCopyBytes(hh), nil
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
