package etcd

import (
	bytes "bytes"
	"path"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/sensu/sensu-go/backend/store"
)

type Comparator struct {
	predicates []predicate
}

func Comparisons(comparisons ...predicate) *Comparator {
	comparator := &Comparator{}
	for _, predicate := range comparisons {
		if !predicate.IsNil() {
			comparator.predicates = append(comparator.predicates, predicate)
		}
	}
	return comparator
}

func (c *Comparator) Cmp() []clientv3.Cmp {
	comparisons := []clientv3.Cmp{}
	for _, predicate := range c.predicates {
		comparisons = append(comparisons, predicate.Cmp())
	}
	return comparisons
}

func (c *Comparator) Failure() []clientv3.Op {
	operations := []clientv3.Op{}
	for _, predicate := range c.predicates {
		operations = append(operations, predicate.Failure())
	}
	return operations
}

func (c *Comparator) Error(resp *clientv3.TxnResponse) error {
	for i, predicate := range c.predicates {
		// We should always have a response for every predicate
		if i >= len(resp.Responses) {
			return &store.ErrInternal{
				Message: "transaction failed due to comparisons evaluating to false but could not determine the cause",
			}
		}
		if err := predicate.Error(resp.Responses[i]); err != nil {
			return err
		}
	}

	// We should never reach this point, but as safeguard
	return &store.ErrInternal{
		Message: "transaction failed due to comparisons evaluating to false but could not determine the cause",
	}
}

type predicate interface {
	Cmp() clientv3.Cmp
	Failure() clientv3.Op
	Error(resp *etcdserverpb.ResponseOp) error
	IsNil() bool
}

//
// NamespaceExists ensures the provided namespace exists
//
type namespaceExists struct {
	namespace string
}

func NamespaceExists(namespace string) *namespaceExists {
	if namespace == "" {
		return nil
	}
	return &namespaceExists{namespace: namespace}
}

func (n *namespaceExists) Cmp() clientv3.Cmp {
	key := path.Join(EtcdRoot, namespacesPathPrefix, n.namespace)
	return clientv3.Compare(
		clientv3.CreateRevision(key), ">", 0,
	)
}

func (n *namespaceExists) Failure() clientv3.Op {
	key := path.Join(EtcdRoot, namespacesPathPrefix, n.namespace)
	return clientv3.OpGet(key)
}

func (n *namespaceExists) Error(resp *etcdserverpb.ResponseOp) error {
	if resp.GetResponseRange().Count == 0 {
		return &store.ErrNamespaceMissing{Namespace: n.namespace}
	}
	return nil
}

func (n *namespaceExists) IsNil() bool {
	return n == nil
}

//
// keyHasValue ensures the provided key has the given value
//
type keyHasValue struct {
	name  string
	value []byte
}

func KeyHasValue(name string, value []byte) *keyHasValue {
	if name == "" {
		return nil
	}
	return &keyHasValue{name: name, value: value}
}

func (k *keyHasValue) Cmp() clientv3.Cmp {
	return clientv3.Compare(clientv3.Value(k.name), "=", string(k.value))
}

func (k *keyHasValue) Failure() clientv3.Op {
	return clientv3.OpGet(k.name)
}

func (k *keyHasValue) Error(resp *etcdserverpb.ResponseOp) error {
	if !bytes.Equal(resp.GetResponseRange().Kvs[0].Value, k.value) {
		return &store.ErrPreconditionFailed{Key: k.name}
	}
	return nil
}

func (k *keyHasValue) IsNil() bool {
	return k == nil
}

//
// keyIsFound ensures the provided key does not exist
//
type keyIsFound struct {
	name string
}

func KeyIsFound(name string) *keyIsFound {
	if name == "" {
		return nil
	}
	return &keyIsFound{name: name}
}

func (k *keyIsFound) Cmp() clientv3.Cmp {
	return clientv3.Compare(
		clientv3.CreateRevision(k.name), ">", 0,
	)
}

func (k *keyIsFound) Failure() clientv3.Op {
	return clientv3.OpGet(k.name)
}

func (k *keyIsFound) Error(resp *etcdserverpb.ResponseOp) error {
	if resp.GetResponseRange().Count == 0 {
		return &store.ErrNotFound{Key: k.name}
	}
	return nil
}

func (k *keyIsFound) IsNil() bool {
	return k == nil
}

//
// keyIsNotFound ensures the provided key does not exist
//
type keyIsNotFound struct {
	name string
}

func KeyIsNotFound(name string) *keyIsNotFound {
	if name == "" {
		return nil
	}
	return &keyIsNotFound{name: name}
}

func (k *keyIsNotFound) Cmp() clientv3.Cmp {
	return clientv3.Compare(
		clientv3.CreateRevision(k.name), "=", 0,
	)
}

func (k *keyIsNotFound) Failure() clientv3.Op {
	return clientv3.OpGet(k.name)
}

func (k *keyIsNotFound) Error(resp *etcdserverpb.ResponseOp) error {
	if resp.GetResponseRange().Count != 0 {
		return &store.ErrAlreadyExists{Key: k.name}
	}
	return nil
}

func (k *keyIsNotFound) IsNil() bool {
	return k == nil
}
