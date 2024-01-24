/*
SPDX-License-Identifier: Apache-2.0

Copyright Contributors to the Submariner project.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// For reference:
// https://github.com/submariner-io/submariner/blob/v0.15.1/pkg/iptables/iptables.go

package iptables

import (
	"os"

	"github.com/coreos/go-iptables/iptables"
	"github.com/pkg/errors"
	"k8s.io/klog"
	"k8s.io/utils/exec"
)

type Basic interface {
	Append(table, chain string, rulespec ...string) error
	AppendUnique(table, chain string, rulespec ...string) error
	Delete(table, chain string, rulespec ...string) error
	Insert(table, chain string, pos int, rulespec ...string) error
	List(table, chain string) ([]string, error)
	ListChains(table string) ([]string, error)
	NewChain(table, chain string) error
	ChainExists(table, chain string) (bool, error)
	ClearChain(table, chain string) error
	DeleteChain(table, chain string) error
}

type Interface interface {
	Basic
	CreateChainIfNotExists(table, chain string) error
	InsertUnique(table, chain string, position int, ruleSpec []string) error
	PrependUnique(table, chain string, ruleSpec []string) error
	// UpdateChainRules ensures that the rules in the list are the ones in rules, without any preference for the order,
	// any stale rules will be removed from the chain, and any missing rules will be added.
	UpdateChainRules(table, chain string, rules [][]string) error
}

type iptablesWrapper struct {
	*iptables.IPTables
}

var NewFunc func() (Interface, error)

// useful for tencent TKE
func init() {
	errInfo := "select iptables-nft or iptables-legacy error"
	execInterface := exec.New()
	ret_nft, err := execInterface.Command("iptables-nft-save").CombinedOutput()
	if err != nil {
		klog.Errorf("%s: %v", errInfo, err)
		return
	}
	ret_legacy, err := execInterface.Command("iptables-legacy-save").CombinedOutput()
	if err != nil {
		klog.Errorf("%s: %v", errInfo, err)
		return
	}
	if len(ret_nft) > len(ret_legacy) {
		err := os.Setenv("IPTABLES_PATH", "/sbin/xtables-nft-multi")
		if err != nil {
			klog.Errorf("%s, set env error: %v", errInfo, err)
			return
		}
	}
}

func New(proto iptables.Protocol) (Interface, error) {
	if NewFunc != nil {
		return NewFunc()
	}

	// IPTABLES_PATH: the path decision the model of iptable, /sbin/xtables-nft-multi => nf_tables
	ipt, err := iptables.New(iptables.IPFamily(proto), iptables.Timeout(5), iptables.Path(os.Getenv("IPTABLES_PATH")))
	if err != nil {
		return nil, errors.Wrap(err, "error creating IP tables")
	}

	return &Adapter{Basic: &iptablesWrapper{IPTables: ipt}}, nil
}

func (i *iptablesWrapper) Delete(table, chain string, rulespec ...string) error {
	err := i.IPTables.Delete(table, chain, rulespec...)

	var iptError *iptables.Error

	ok := errors.As(err, &iptError)
	if ok && iptError.IsNotExist() {
		return nil
	}

	return errors.Wrap(err, "error deleting IP table rule")
}
