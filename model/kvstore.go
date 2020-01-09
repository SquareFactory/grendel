package model

import (
	"fmt"

	"github.com/segmentio/ksuid"
	"github.com/timshannon/badgerhold"
	"github.com/ubccr/grendel/nodeset"
)

type KVStore struct {
	store *badgerhold.Store
}

func NewKVStore(filename string) (*KVStore, error) {
	options := badgerhold.DefaultOptions
	options.Dir = filename
	options.ValueDir = filename
	options.Logger = log
	store, err := badgerhold.Open(options)
	if err != nil {
		return nil, err
	}

	return &KVStore{store: store}, nil
}

func (s *KVStore) GetBootImage(mac string) (*BootImage, error) {
	return nil, nil
}

func (s *KVStore) GetHostByID(id string) (*Host, error) {
	uuid, err := ksuid.Parse(id)
	if err != nil {
		return nil, err
	}

	host := &Host{}

	err = s.store.Get(uuid.Bytes(), host)
	if err != nil {
		return nil, err
	}

	return host, nil
}

func (s *KVStore) GetHostByName(name string) (*Host, error) {
	host := make([]*Host, 0)

	err := s.store.Find(&host, badgerhold.Where("Name").Eq(name))
	if err != nil {
		return nil, err
	}

	if len(host) == 0 {
		return nil, fmt.Errorf("Host not found with name: %s", name)
	}

	if len(host) > 1 {
		log.Warnf("Multiple hosts found with nam name: %s", name)
	}

	return host[0], nil
}

func (s *KVStore) SaveHost(host *Host) error {
	if host.ID.IsNil() {
		uuid, err := ksuid.NewRandom()
		if err != nil {
			return err
		}
		host.ID = uuid
	}

	return s.store.Upsert(host.ID.Bytes(), host)
}

func (s *KVStore) HostList() (HostList, error) {
	var result HostList

	err := s.store.Find(&result, nil)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *KVStore) Find(ns *nodeset.NodeSet) (HostList, error) {
	values := make(HostList, 0)

	it := ns.Iterator()
	for it.Next() {
		host, err := s.GetHostByName(it.Value())
		if err == nil {
			values = append(values, host)
		}
	}

	return values, nil
}

func (s *KVStore) Close() error {
	return s.store.Close()
}
