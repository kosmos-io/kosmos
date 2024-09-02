package ironicparametercontroller

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/kosmos.io/kosmos/pkg/kubenest/controller/virtualcluster.node.controller/exector"
)

func getIronicNpuTorJsonString() string {
	return `
	{
        "NodeName": "node-00",
        "IronicDeviceId": "bbef4fca-6ecd-49f6-98de-5efda5d315b6",
        "IronicPortId": "63beecbb-67e2-4ae0-b369-c9eae4898ce2",
        "TorSwitchIpList": [
            "10.150.241.37",
            "10.150.241.37",
            "10.150.241.37",
            "10.150.241.37",
            "10.150.241.37",
            "10.150.241.37",
            "10.150.241.37",
            "10.150.241.37"
        ],
        "FixIp": "192.168.2.3",
        "NpuInfo": [
            {
                "id": "f75bbcec-ce33-4ce8-be39-5e7e381b6b8c",
                "switch_ip": "10.150.241.37",
                "mac_address": "98:f0:83:d7:26:ee",
                "port_name": "200GE1/0/10",
                "ip_address": "10.100.6.2",
                "gateway": "10.100.6.1",
                "mask": 24
            },
            {
                "id": "0cb3beba-bcc3-4e45-8618-ff009860e30e",
                "switch_ip": "10.150.241.37",
                "mac_address": "98:f0:83:d7:27:13",
                "port_name": "200GE1/0/9",
                "ip_address": "10.100.6.3",
                "gateway": "10.100.6.1",
                "mask": 24
            },
            {
                "id": "d6a1d2eb-e427-4762-974d-c96feb91aee8",
                "switch_ip": "10.150.241.37",
                "mac_address": "98:f0:83:d7:27:14",
                "port_name": "200GE1/0/12",
                "ip_address": "10.100.6.4",
                "gateway": "10.100.6.1",
                "mask": 24
            },
            {
                "id": "24c5a355-c390-4892-b490-03faf22dabd8",
                "switch_ip": "10.150.241.37",
                "mac_address": "98:f0:83:d7:27:08",
                "port_name": "200GE1/0/11",
                "ip_address": "10.100.6.5",
                "gateway": "10.100.6.1",
                "mask": 24
            },
            {
                "id": "32eeeb22-b052-4407-b16b-b579e6c8f401",
                "switch_ip": "10.150.241.37",
                "mac_address": "98:f0:83:d7:26:f6",
                "port_name": "200GE1/0/16",
                "ip_address": "10.100.6.6",
                "gateway": "10.100.6.1",
                "mask": 24
            },
            {
                "id": "8f74b93a-f7fb-4b4c-8eb4-dc1e502ff538",
                "switch_ip": "10.150.241.37",
                "mac_address": "98:f0:83:d7:26:d5",
                "port_name": "200GE1/0/15",
                "ip_address": "10.100.6.7",
                "gateway": "10.100.6.1",
                "mask": 24
            },
            {
                "id": "ad956b56-7836-4b56-a51c-bddcf8f7f257",
                "switch_ip": "10.150.241.37",
                "mac_address": "98:f0:83:d7:26:d3",
                "port_name": "200GE1/0/14",
                "ip_address": "10.100.6.8",
                "gateway": "10.100.6.1",
                "mask": 24
            },
            {
                "id": "a3c148ec-1704-4e1a-aede-2dd94703d7cb",
                "switch_ip": "10.150.241.37",
                "mac_address": "98:f0:83:d7:26:d7",
                "port_name": "200GE1/0/13",
                "ip_address": "10.100.6.9",
                "gateway": "10.100.6.1",
                "mask": 24
            }
        ],
        "NpuDeviceIpList": [
            "10.100.6.2",
            "10.100.6.3",
            "10.100.6.4",
            "10.100.6.5",
            "10.100.6.6",
            "10.100.6.7",
            "10.100.6.8",
            "10.100.6.9"
        ]
    }
	`
}

func getIronicNpuTor() (*IronicNpuTor, error) {
	var ironicNpuTour IronicNpuTor
	ironicNpuTorJson := getIronicNpuTorJsonString()
	err := json.Unmarshal([]byte(ironicNpuTorJson), &ironicNpuTour)
	if err != nil {
		return nil, err
	}
	return &ironicNpuTour, nil
}

func fakeDoTask(ctx context.Context, ironicNpuTour *IronicNpuTor) error {
	i := IronicParameterController{}
	nodeIP := "127.0.0.1"
	exectHelper := exector.NewExectorHelperForHccn(nodeIP, "")

	macs, err := i.GetNodeMacList(ctx, exectHelper)
	if err != nil {
		return fmt.Errorf("do task failed, nodename: %s, err: %s", ironicNpuTour.NodeName, err.Error())
	}

	err = i.SetNetwork(ctx, exectHelper, macs, ironicNpuTour.NpuInfo)

	if err != nil {
		return fmt.Errorf("do task failed, nodename: %s, err: %s", ironicNpuTour.NodeName, err.Error())
	}
	return nil
}

func TestIronicParameterControllerDoTask(t *testing.T) {
	_ = os.Setenv("WEB_USER", "node")
	_ = os.Setenv("WEB_PASS", "de1ec_ooDaiceiJu")

	var ironicNpuTour *IronicNpuTor
	var err error
	if ironicNpuTour, err = getIronicNpuTor(); err != nil {
		t.Fatalf("get ironicNpuTor failed, err: %v", err)
	}

	if err := fakeDoTask(context.TODO(), ironicNpuTour); err != nil {
		t.Fatalf("DoTask failed, err: %v", err)
	}
}
