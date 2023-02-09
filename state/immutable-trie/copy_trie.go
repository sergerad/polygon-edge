package itrie

import (
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/0xPolygon/polygon-edge/state"
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/umbracle/fastrlp"
)

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
			if account.CodeHash != nil {
				code, ok := storage.GetCode(types.BytesToHash(account.CodeHash))
				if !ok {
					fmt.Println("------------------------Code is empty-------------")
				}
				newStorage.SetCode(types.BytesToHash(account.CodeHash), code)
			}

			if account.Root != types.EmptyRootHash {
				err = CopyTrie1(account.Root[:], storage, newStorage, nil)
				return err
			}
		}

	case *ShortNode:
		err := CopyTrie(n.child, storage, newStorage, append(agg, n.key...))
		if err != nil {
			return err
		}
	}
	return nil
}

func HashChecker1(node Node, storage Storage) (types.Hash, error) {
	h, ok := hasherPool.Get().(*hasher)
	if !ok {
		return types.Hash{}, errors.New("cant get hasher")
	}
	arena, _ := h.AcquireArena()
	val, err := HashChecker(node, h, arena, 0, storage)
	if err != nil {
		return types.Hash{}, err
	}

	h.ReleaseArenas(0)
	hasherPool.Put(h)
	return types.BytesToHash(val.Raw()), nil
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

func NewKV(db *leveldb.DB) *KVStorage {
	return &KVStorage{db: db}
}

func NewTrieWithRoot(root Node) *Trie {
	return &Trie{
		root: root,
	}
}
