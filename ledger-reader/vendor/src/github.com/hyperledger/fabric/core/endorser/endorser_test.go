/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package endorser

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/golang/protobuf/proto"
	mc "github.com/hyperledger/fabric/common/mocks/config"
	"github.com/hyperledger/fabric/common/mocks/resourcesconfig"
	"github.com/hyperledger/fabric/common/util"
	"github.com/hyperledger/fabric/core/endorser/mocks"
	"github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/core/mocks/ccprovider"
	em "github.com/hyperledger/fabric/core/mocks/endorser"
	"github.com/hyperledger/fabric/msp"
	"github.com/hyperledger/fabric/msp/mgmt"
	"github.com/hyperledger/fabric/msp/mgmt/testtools"
	"github.com/hyperledger/fabric/protos/common"
	"github.com/hyperledger/fabric/protos/ledger/rwset"
	pb "github.com/hyperledger/fabric/protos/peer"
	"github.com/hyperledger/fabric/protos/utils"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func getSignedPropWithCHID(ccid, ccver, chid string, t *testing.T) *pb.SignedProposal {
	ccargs := [][]byte{[]byte("args")}

	return getSignedPropWithCHIdAndArgs(chid, ccid, ccver, ccargs, t)
}

func getSignedProp(ccid, ccver string, t *testing.T) *pb.SignedProposal {
	return getSignedPropWithCHID(ccid, ccver, util.GetTestChainID(), t)
}

func getSignedPropWithCHIdAndArgs(chid, ccid, ccver string, ccargs [][]byte, t *testing.T) *pb.SignedProposal {
	spec := &pb.ChaincodeSpec{Type: 1, ChaincodeId: &pb.ChaincodeID{Name: ccid, Version: ccver}, Input: &pb.ChaincodeInput{Args: ccargs}}

	cis := &pb.ChaincodeInvocationSpec{ChaincodeSpec: spec}

	creator, err := signer.Serialize()
	assert.NoError(t, err)
	prop, _, err := utils.CreateChaincodeProposal(common.HeaderType_ENDORSER_TRANSACTION, chid, cis, creator)
	assert.NoError(t, err)
	propBytes, err := utils.GetBytesProposal(prop)
	assert.NoError(t, err)
	signature, err := signer.Sign(propBytes)
	assert.NoError(t, err)
	return &pb.SignedProposal{ProposalBytes: propBytes, Signature: signature}
}

func TestEndorserNilProp(t *testing.T) {
	es := NewEndorserServer(func(channel string, txID string, privateData *rwset.TxPvtReadWriteSet) error {
		return nil
	}, &em.MockSupport{
		GetApplicationConfigBoolRv: true,
		GetApplicationConfigRv:     &mc.MockApplication{&mc.MockApplicationCapabilities{}},
		GetTransactionByIDErr:      errors.New(""),
		ChaincodeDefinitionRv:      &resourceconfig.MockChaincodeDefinition{EndorsementStr: "ESCC"},
		ExecuteResp:                &pb.Response{Status: 200, Payload: utils.MarshalOrPanic(&pb.ProposalResponse{Response: &pb.Response{}})},
		GetTxSimulatorRv:           &ccprovider.MockTxSim{&ledger.TxSimulationResults{PubSimulationResults: &rwset.TxReadWriteSet{}}},
	})

	_, err := es.ProcessProposal(context.Background(), nil)
	assert.Error(t, err)
}

func TestEndorserUninvokableSysCC(t *testing.T) {
	es := NewEndorserServer(func(channel string, txID string, privateData *rwset.TxPvtReadWriteSet) error {
		return nil
	}, &em.MockSupport{
		GetApplicationConfigBoolRv:       true,
		GetApplicationConfigRv:           &mc.MockApplication{&mc.MockApplicationCapabilities{}},
		GetTransactionByIDErr:            errors.New(""),
		IsSysCCAndNotInvokableExternalRv: true,
	})

	signedProp := getSignedProp("ccid", "0", t)

	_, err := es.ProcessProposal(context.Background(), signedProp)
	assert.Error(t, err)
}

func TestEndorserCCInvocationFailed(t *testing.T) {
	es := NewEndorserServer(func(channel string, txID string, privateData *rwset.TxPvtReadWriteSet) error {
		return nil
	}, &em.MockSupport{
		GetApplicationConfigBoolRv: true,
		GetApplicationConfigRv:     &mc.MockApplication{&mc.MockApplicationCapabilities{}},
		GetTransactionByIDErr:      errors.New(""),
		ChaincodeDefinitionRv:      &resourceconfig.MockChaincodeDefinition{EndorsementStr: "ESCC"},
		ExecuteResp:                &pb.Response{Status: 1000, Payload: utils.MarshalOrPanic(&pb.ProposalResponse{Response: &pb.Response{}})},
		GetTxSimulatorRv:           &ccprovider.MockTxSim{&ledger.TxSimulationResults{PubSimulationResults: &rwset.TxReadWriteSet{}}},
	})

	signedProp := getSignedProp("ccid", "0", t)

	_, err := es.ProcessProposal(context.Background(), signedProp)
	assert.Error(t, err)
}

func TestEndorserNoCCDef(t *testing.T) {
	es := NewEndorserServer(func(channel string, txID string, privateData *rwset.TxPvtReadWriteSet) error {
		return nil
	}, &em.MockSupport{
		GetApplicationConfigBoolRv: true,
		GetApplicationConfigRv:     &mc.MockApplication{&mc.MockApplicationCapabilities{}},
		GetTransactionByIDErr:      errors.New(""),
		ChaincodeDefinitionError:   errors.New(""),
		ExecuteResp:                &pb.Response{Status: 200, Payload: utils.MarshalOrPanic(&pb.ProposalResponse{Response: &pb.Response{}})},
		GetTxSimulatorRv:           &ccprovider.MockTxSim{&ledger.TxSimulationResults{PubSimulationResults: &rwset.TxReadWriteSet{}}},
	})

	signedProp := getSignedProp("ccid", "0", t)

	_, err := es.ProcessProposal(context.Background(), signedProp)
	assert.Error(t, err)
}

func TestEndorserBadInstPolicy(t *testing.T) {
	es := NewEndorserServer(func(channel string, txID string, privateData *rwset.TxPvtReadWriteSet) error {
		return nil
	}, &em.MockSupport{
		GetApplicationConfigBoolRv:    true,
		GetApplicationConfigRv:        &mc.MockApplication{&mc.MockApplicationCapabilities{}},
		GetTransactionByIDErr:         errors.New(""),
		CheckInstantiationPolicyError: errors.New(""),
		ChaincodeDefinitionRv:         &resourceconfig.MockChaincodeDefinition{EndorsementStr: "ESCC"},
		ExecuteResp:                   &pb.Response{Status: 200, Payload: utils.MarshalOrPanic(&pb.ProposalResponse{Response: &pb.Response{}})},
		GetTxSimulatorRv:              &ccprovider.MockTxSim{&ledger.TxSimulationResults{PubSimulationResults: &rwset.TxReadWriteSet{}}},
	})

	signedProp := getSignedProp("ccid", "0", t)

	_, err := es.ProcessProposal(context.Background(), signedProp)
	assert.Error(t, err)
}

func TestEndorserSysCC(t *testing.T) {
	es := NewEndorserServer(func(channel string, txID string, privateData *rwset.TxPvtReadWriteSet) error {
		return nil
	}, &em.MockSupport{
		GetApplicationConfigBoolRv: true,
		GetApplicationConfigRv:     &mc.MockApplication{&mc.MockApplicationCapabilities{}},
		GetTransactionByIDErr:      errors.New(""),
		IsSysCCRv:                  true,
		ChaincodeDefinitionRv:      &resourceconfig.MockChaincodeDefinition{EndorsementStr: "ESCC"},
		ExecuteResp:                &pb.Response{Status: 200, Payload: utils.MarshalOrPanic(&pb.ProposalResponse{Response: &pb.Response{}})},
		GetTxSimulatorRv:           &ccprovider.MockTxSim{&ledger.TxSimulationResults{PubSimulationResults: &rwset.TxReadWriteSet{}}},
	})

	signedProp := getSignedProp("ccid", "0", t)

	_, err := es.ProcessProposal(context.Background(), signedProp)
	assert.NoError(t, err)
}

func TestEndorserCCInvocationError(t *testing.T) {
	es := NewEndorserServer(func(channel string, txID string, privateData *rwset.TxPvtReadWriteSet) error {
		return nil
	}, &em.MockSupport{
		GetApplicationConfigBoolRv: true,
		GetApplicationConfigRv:     &mc.MockApplication{&mc.MockApplicationCapabilities{}},
		GetTransactionByIDErr:      errors.New(""),
		ExecuteError:               errors.New(""),
		ChaincodeDefinitionRv:      &resourceconfig.MockChaincodeDefinition{EndorsementStr: "ESCC"},
		GetTxSimulatorRv:           &ccprovider.MockTxSim{&ledger.TxSimulationResults{PubSimulationResults: &rwset.TxReadWriteSet{}}},
	})

	signedProp := getSignedProp("ccid", "0", t)

	_, err := es.ProcessProposal(context.Background(), signedProp)
	assert.Error(t, err)
}

func TestEndorserLSCCBadType(t *testing.T) {
	es := NewEndorserServer(func(channel string, txID string, privateData *rwset.TxPvtReadWriteSet) error {
		return nil
	}, &em.MockSupport{
		GetApplicationConfigBoolRv: true,
		GetApplicationConfigRv:     &mc.MockApplication{&mc.MockApplicationCapabilities{}},
		GetTransactionByIDErr:      errors.New(""),
		ChaincodeDefinitionRv:      &resourceconfig.MockChaincodeDefinition{EndorsementStr: "ESCC"},
		ExecuteResp:                &pb.Response{Status: 200, Payload: utils.MarshalOrPanic(&pb.ProposalResponse{Response: &pb.Response{}})},
		GetTxSimulatorRv:           &ccprovider.MockTxSim{&ledger.TxSimulationResults{PubSimulationResults: &rwset.TxReadWriteSet{}}},
	})

	cds := utils.MarshalOrPanic(
		&pb.ChaincodeDeploymentSpec{
			ChaincodeSpec: &pb.ChaincodeSpec{
				ChaincodeId: &pb.ChaincodeID{Name: "barf"},
				Type:        pb.ChaincodeSpec_UNDEFINED,
			},
		},
	)
	signedProp := getSignedPropWithCHIdAndArgs(util.GetTestChainID(), "lscc", "0", [][]byte{[]byte("deploy"), []byte("a"), cds}, t)

	_, err := es.ProcessProposal(context.Background(), signedProp)
	assert.Error(t, err)
}

func TestEndorserDupTXId(t *testing.T) {
	es := NewEndorserServer(func(channel string, txID string, privateData *rwset.TxPvtReadWriteSet) error {
		return nil
	}, &em.MockSupport{
		GetApplicationConfigBoolRv: true,
		GetApplicationConfigRv:     &mc.MockApplication{&mc.MockApplicationCapabilities{}},
		ChaincodeDefinitionRv:      &resourceconfig.MockChaincodeDefinition{EndorsementStr: "ESCC"},
		ExecuteResp:                &pb.Response{Status: 200, Payload: utils.MarshalOrPanic(&pb.ProposalResponse{Response: &pb.Response{}})},
		GetTxSimulatorRv:           &ccprovider.MockTxSim{&ledger.TxSimulationResults{PubSimulationResults: &rwset.TxReadWriteSet{}}},
	})

	signedProp := getSignedProp("ccid", "0", t)

	_, err := es.ProcessProposal(context.Background(), signedProp)
	assert.Error(t, err)
}

func TestEndorserBadACL(t *testing.T) {
	es := NewEndorserServer(func(channel string, txID string, privateData *rwset.TxPvtReadWriteSet) error {
		return nil
	}, &em.MockSupport{
		GetApplicationConfigBoolRv: true,
		GetApplicationConfigRv:     &mc.MockApplication{&mc.MockApplicationCapabilities{}},
		CheckACLErr:                errors.New(""),
		GetTransactionByIDErr:      errors.New(""),
		ChaincodeDefinitionRv:      &resourceconfig.MockChaincodeDefinition{EndorsementStr: "ESCC"},
		ExecuteResp:                &pb.Response{Status: 200, Payload: utils.MarshalOrPanic(&pb.ProposalResponse{Response: &pb.Response{}})},
		GetTxSimulatorRv:           &ccprovider.MockTxSim{&ledger.TxSimulationResults{PubSimulationResults: &rwset.TxReadWriteSet{}}},
	})

	signedProp := getSignedProp("ccid", "0", t)

	_, err := es.ProcessProposal(context.Background(), signedProp)
	assert.Error(t, err)
}

func TestEndorserGoodPathEmptyChannel(t *testing.T) {
	es := NewEndorserServer(func(channel string, txID string, privateData *rwset.TxPvtReadWriteSet) error {
		return nil
	}, &em.MockSupport{
		GetApplicationConfigBoolRv: true,
		GetApplicationConfigRv:     &mc.MockApplication{&mc.MockApplicationCapabilities{}},
		GetTransactionByIDErr:      errors.New(""),
		ChaincodeDefinitionRv:      &resourceconfig.MockChaincodeDefinition{EndorsementStr: "ESCC"},
		ExecuteResp:                &pb.Response{Status: 200, Payload: utils.MarshalOrPanic(&pb.ProposalResponse{Response: &pb.Response{}})},
		GetTxSimulatorRv:           &ccprovider.MockTxSim{&ledger.TxSimulationResults{PubSimulationResults: &rwset.TxReadWriteSet{}}},
	})

	signedProp := getSignedPropWithCHIdAndArgs("", "ccid", "0", [][]byte{[]byte("args")}, t)

	_, err := es.ProcessProposal(context.Background(), signedProp)
	assert.NoError(t, err)
}

func TestEndorserLSCCInitFails(t *testing.T) {
	es := NewEndorserServer(func(channel string, txID string, privateData *rwset.TxPvtReadWriteSet) error {
		return nil
	}, &em.MockSupport{
		GetApplicationConfigBoolRv: true,
		GetApplicationConfigRv:     &mc.MockApplication{&mc.MockApplicationCapabilities{}},
		GetTransactionByIDErr:      errors.New(""),
		ChaincodeDefinitionRv:      &resourceconfig.MockChaincodeDefinition{EndorsementStr: "ESCC"},
		ExecuteResp:                &pb.Response{Status: 200, Payload: utils.MarshalOrPanic(&pb.ProposalResponse{Response: &pb.Response{}})},
		GetTxSimulatorRv:           &ccprovider.MockTxSim{&ledger.TxSimulationResults{PubSimulationResults: &rwset.TxReadWriteSet{}}},
		ExecuteCDSError:            errors.New(""),
	})

	cds := utils.MarshalOrPanic(
		&pb.ChaincodeDeploymentSpec{
			ChaincodeSpec: &pb.ChaincodeSpec{
				ChaincodeId: &pb.ChaincodeID{Name: "barf"},
				Type:        pb.ChaincodeSpec_GOLANG,
			},
		},
	)
	signedProp := getSignedPropWithCHIdAndArgs(util.GetTestChainID(), "lscc", "0", [][]byte{[]byte("deploy"), []byte("a"), cds}, t)

	_, err := es.ProcessProposal(context.Background(), signedProp)
	assert.Error(t, err)
}

func TestEndorserLSCCDeploySysCC(t *testing.T) {
	SysCCMap := make(map[string]struct{})
	deployedCCName := "barf"
	SysCCMap[deployedCCName] = struct{}{}
	es := NewEndorserServer(func(channel string, txID string, privateData *rwset.TxPvtReadWriteSet) error {
		return nil
	}, &em.MockSupport{
		GetApplicationConfigBoolRv: true,
		GetApplicationConfigRv:     &mc.MockApplication{&mc.MockApplicationCapabilities{}},
		GetTransactionByIDErr:      errors.New(""),
		ChaincodeDefinitionRv:      &resourceconfig.MockChaincodeDefinition{EndorsementStr: "ESCC"},
		ExecuteResp:                &pb.Response{Status: 200, Payload: utils.MarshalOrPanic(&pb.ProposalResponse{Response: &pb.Response{}})},
		GetTxSimulatorRv:           &ccprovider.MockTxSim{&ledger.TxSimulationResults{PubSimulationResults: &rwset.TxReadWriteSet{}}},
		SysCCMap:                   SysCCMap,
	})

	cds := utils.MarshalOrPanic(
		&pb.ChaincodeDeploymentSpec{
			ChaincodeSpec: &pb.ChaincodeSpec{
				ChaincodeId: &pb.ChaincodeID{Name: deployedCCName},
				Type:        pb.ChaincodeSpec_GOLANG,
			},
		},
	)
	signedProp := getSignedPropWithCHIdAndArgs(util.GetTestChainID(), "lscc", "0", [][]byte{[]byte("deploy"), []byte("a"), cds}, t)

	_, err := es.ProcessProposal(context.Background(), signedProp)
	assert.Error(t, err)
}

func TestEndorserLSCCJava1(t *testing.T) {
	if javaEnabled() {
		t.Skip("Java chaincode is supported")
	}

	es := NewEndorserServer(func(channel string, txID string, privateData *rwset.TxPvtReadWriteSet) error {
		return nil
	}, &em.MockSupport{
		GetApplicationConfigBoolRv: true,
		GetApplicationConfigRv:     &mc.MockApplication{&mc.MockApplicationCapabilities{}},
		IsJavaRV:                   true,
		GetTransactionByIDErr:      errors.New(""),
		ChaincodeDefinitionRv:      &resourceconfig.MockChaincodeDefinition{EndorsementStr: "ESCC"},
		ExecuteResp:                &pb.Response{Status: 200, Payload: utils.MarshalOrPanic(&pb.ProposalResponse{Response: &pb.Response{}})},
		GetTxSimulatorRv:           &ccprovider.MockTxSim{&ledger.TxSimulationResults{PubSimulationResults: &rwset.TxReadWriteSet{}}},
	})

	cds := utils.MarshalOrPanic(
		&pb.ChaincodeDeploymentSpec{
			ChaincodeSpec: &pb.ChaincodeSpec{
				ChaincodeId: &pb.ChaincodeID{Name: "barf"},
				Type:        pb.ChaincodeSpec_JAVA,
			},
		},
	)
	signedProp := getSignedPropWithCHIdAndArgs(util.GetTestChainID(), "lscc", "0", [][]byte{[]byte("deploy"), []byte("a"), cds}, t)

	_, err := es.ProcessProposal(context.Background(), signedProp)
	assert.Error(t, err)
}

func TestEndorserLSCCJava2(t *testing.T) {
	if javaEnabled() {
		t.Skip("Java chaincode is supported")
	}

	es := NewEndorserServer(func(channel string, txID string, privateData *rwset.TxPvtReadWriteSet) error {
		return nil
	}, &em.MockSupport{
		GetApplicationConfigBoolRv: true,
		GetApplicationConfigRv:     &mc.MockApplication{&mc.MockApplicationCapabilities{}},
		IsJavaErr:                  errors.New(""),
		GetTransactionByIDErr:      errors.New(""),
		ChaincodeDefinitionRv:      &resourceconfig.MockChaincodeDefinition{EndorsementStr: "ESCC"},
		ExecuteResp:                &pb.Response{Status: 200, Payload: utils.MarshalOrPanic(&pb.ProposalResponse{Response: &pb.Response{}})},
		GetTxSimulatorRv:           &ccprovider.MockTxSim{&ledger.TxSimulationResults{PubSimulationResults: &rwset.TxReadWriteSet{}}},
	})

	cds := utils.MarshalOrPanic(
		&pb.ChaincodeDeploymentSpec{
			ChaincodeSpec: &pb.ChaincodeSpec{
				ChaincodeId: &pb.ChaincodeID{Name: "barf"},
				Type:        pb.ChaincodeSpec_JAVA,
			},
		},
	)
	signedProp := getSignedPropWithCHIdAndArgs(util.GetTestChainID(), "lscc", "0", [][]byte{[]byte("deploy"), []byte("a"), cds}, t)

	_, err := es.ProcessProposal(context.Background(), signedProp)
	assert.Error(t, err)
}

func TestEndorserGoodPathWEvents(t *testing.T) {
	es := NewEndorserServer(func(channel string, txID string, privateData *rwset.TxPvtReadWriteSet) error {
		return nil
	}, &em.MockSupport{
		GetApplicationConfigBoolRv: true,
		GetApplicationConfigRv:     &mc.MockApplication{&mc.MockApplicationCapabilities{}},
		GetTransactionByIDErr:      errors.New(""),
		ChaincodeDefinitionRv:      &resourceconfig.MockChaincodeDefinition{EndorsementStr: "ESCC"},
		ExecuteResp:                &pb.Response{Status: 200, Payload: utils.MarshalOrPanic(&pb.ProposalResponse{Response: &pb.Response{}})},
		ExecuteEvent:               &pb.ChaincodeEvent{},
		GetTxSimulatorRv:           &ccprovider.MockTxSim{&ledger.TxSimulationResults{PubSimulationResults: &rwset.TxReadWriteSet{}}},
	})

	signedProp := getSignedProp("ccid", "0", t)

	_, err := es.ProcessProposal(context.Background(), signedProp)
	assert.NoError(t, err)
}

func TestEndorserBadChannel(t *testing.T) {
	es := NewEndorserServer(func(channel string, txID string, privateData *rwset.TxPvtReadWriteSet) error {
		return nil
	}, &em.MockSupport{
		GetApplicationConfigBoolRv: true,
		GetApplicationConfigRv:     &mc.MockApplication{&mc.MockApplicationCapabilities{}},
		GetTransactionByIDErr:      errors.New(""),
		ChaincodeDefinitionRv:      &resourceconfig.MockChaincodeDefinition{EndorsementStr: "ESCC"},
		ExecuteResp:                &pb.Response{Status: 200, Payload: utils.MarshalOrPanic(&pb.ProposalResponse{Response: &pb.Response{}})},
		GetTxSimulatorRv:           &ccprovider.MockTxSim{&ledger.TxSimulationResults{PubSimulationResults: &rwset.TxReadWriteSet{}}},
	})

	signedProp := getSignedPropWithCHID("ccid", "0", "barfchain", t)

	_, err := es.ProcessProposal(context.Background(), signedProp)
	assert.Error(t, err)
}

func TestEndorserGoodPath(t *testing.T) {
	es := NewEndorserServer(func(channel string, txID string, privateData *rwset.TxPvtReadWriteSet) error {
		return nil
	}, &em.MockSupport{
		GetApplicationConfigBoolRv: true,
		GetApplicationConfigRv:     &mc.MockApplication{&mc.MockApplicationCapabilities{}},
		GetTransactionByIDErr:      errors.New(""),
		ChaincodeDefinitionRv:      &resourceconfig.MockChaincodeDefinition{EndorsementStr: "ESCC"},
		ExecuteResp:                &pb.Response{Status: 200, Payload: utils.MarshalOrPanic(&pb.ProposalResponse{Response: &pb.Response{}})},
		GetTxSimulatorRv:           &ccprovider.MockTxSim{&ledger.TxSimulationResults{PubSimulationResults: &rwset.TxReadWriteSet{}}},
	})

	signedProp := getSignedProp("ccid", "0", t)

	_, err := es.ProcessProposal(context.Background(), signedProp)
	assert.NoError(t, err)
}

func TestEndorserLSCC(t *testing.T) {
	es := NewEndorserServer(func(channel string, txID string, privateData *rwset.TxPvtReadWriteSet) error {
		return nil
	}, &em.MockSupport{
		GetApplicationConfigBoolRv: true,
		GetApplicationConfigRv:     &mc.MockApplication{&mc.MockApplicationCapabilities{}},
		GetTransactionByIDErr:      errors.New(""),
		ChaincodeDefinitionRv:      &resourceconfig.MockChaincodeDefinition{EndorsementStr: "ESCC"},
		ExecuteResp:                &pb.Response{Status: 200, Payload: utils.MarshalOrPanic(&pb.ProposalResponse{Response: &pb.Response{}})},
		GetTxSimulatorRv:           &ccprovider.MockTxSim{&ledger.TxSimulationResults{PubSimulationResults: &rwset.TxReadWriteSet{}}},
	})

	cds := utils.MarshalOrPanic(
		&pb.ChaincodeDeploymentSpec{
			ChaincodeSpec: &pb.ChaincodeSpec{
				ChaincodeId: &pb.ChaincodeID{Name: "barf"},
				Type:        pb.ChaincodeSpec_GOLANG,
			},
		},
	)
	signedProp := getSignedPropWithCHIdAndArgs(util.GetTestChainID(), "lscc", "0", [][]byte{[]byte("deploy"), []byte("a"), cds}, t)

	_, err := es.ProcessProposal(context.Background(), signedProp)
	assert.NoError(t, err)
}

func TestSimulateProposal(t *testing.T) {
	es := NewEndorserServer(func(channel string, txID string, privateData *rwset.TxPvtReadWriteSet) error {
		return nil
	}, &em.MockSupport{
		GetApplicationConfigBoolRv: true,
		GetApplicationConfigRv:     &mc.MockApplication{&mc.MockApplicationCapabilities{}},
		GetTransactionByIDErr:      errors.New(""),
		ChaincodeDefinitionRv:      &resourceconfig.MockChaincodeDefinition{EndorsementStr: "ESCC"},
		ExecuteResp:                &pb.Response{Status: 200, Payload: utils.MarshalOrPanic(&pb.ProposalResponse{Response: &pb.Response{}})},
		GetTxSimulatorRv:           &ccprovider.MockTxSim{&ledger.TxSimulationResults{PubSimulationResults: &rwset.TxReadWriteSet{}}},
	})

	_, _, _, _, err := es.(*Endorser).simulateProposal(nil, "", "", nil, nil, nil, nil)
	assert.Error(t, err)
}

func TestEndorserJavaChecks(t *testing.T) {
	if javaEnabled() {
		t.Skip("Java chaincode is supported")
	}

	es := NewEndorserServer(func(channel string, txID string, privateData *rwset.TxPvtReadWriteSet) error {
		return nil
	}, &em.MockSupport{
		GetApplicationConfigBoolRv: true,
		GetApplicationConfigRv:     &mc.MockApplication{&mc.MockApplicationCapabilities{}},
		GetTransactionByIDErr:      errors.New(""),
		ChaincodeDefinitionRv:      &resourceconfig.MockChaincodeDefinition{EndorsementStr: "ESCC"},
		ExecuteResp:                &pb.Response{Status: 200, Payload: utils.MarshalOrPanic(&pb.ProposalResponse{Response: &pb.Response{}})},
		GetTxSimulatorRv:           &ccprovider.MockTxSim{&ledger.TxSimulationResults{PubSimulationResults: &rwset.TxReadWriteSet{}}},
	})

	err := es.(*Endorser).disableJavaCCInst(&pb.ChaincodeID{Name: "lscc"}, &pb.ChaincodeInvocationSpec{})
	assert.NoError(t, err)
	err = es.(*Endorser).disableJavaCCInst(&pb.ChaincodeID{Name: "lscc"}, &pb.ChaincodeInvocationSpec{ChaincodeSpec: &pb.ChaincodeSpec{Input: &pb.ChaincodeInput{}}})
	assert.NoError(t, err)
	err = es.(*Endorser).disableJavaCCInst(&pb.ChaincodeID{Name: "lscc"}, &pb.ChaincodeInvocationSpec{ChaincodeSpec: &pb.ChaincodeSpec{Input: &pb.ChaincodeInput{Args: [][]byte{[]byte("foo")}}}})
	assert.NoError(t, err)
	err = es.(*Endorser).disableJavaCCInst(&pb.ChaincodeID{Name: "lscc"}, &pb.ChaincodeInvocationSpec{ChaincodeSpec: &pb.ChaincodeSpec{Input: &pb.ChaincodeInput{Args: [][]byte{[]byte("install")}}}})
	assert.Error(t, err)
}

func TestChaincodeError_Error(t *testing.T) {
	ce := &chaincodeError{status: 1, msg: "foo"}
	assert.Equal(t, ce.Error(), "chaincode error (status: 1, message: foo)")
}

func TestEndorserAcquireTxSimulator(t *testing.T) {
	tc := []struct {
		name          string
		chainID       string
		chaincodeName string
		simAcquired   bool
	}{
		{"empty channel", "", "ignored", false},
		{"query scc", util.GetTestChainID(), "qscc", false},
		{"config scc", util.GetTestChainID(), "cscc", false},
		{"mainline", util.GetTestChainID(), "chaincode", true},
	}

	expectedResponse := &pb.Response{Status: 200, Payload: utils.MarshalOrPanic(&pb.ProposalResponse{Response: &pb.Response{}})}
	for _, tt := range tc {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			m := &mock.Mock{}
			mockSim := &ccprovider.MockTxSim{
				GetTxSimulationResultsRv: &ledger.TxSimulationResults{PubSimulationResults: &rwset.TxReadWriteSet{}},
			}
			m.On("GetTxSimulator", mock.Anything, mock.Anything).Return(mockSim, nil)
			support := &em.MockSupport{
				Mock: m,
				GetApplicationConfigBoolRv: true,
				GetApplicationConfigRv:     &mc.MockApplication{CapabilitiesRv: &mc.MockApplicationCapabilities{}},
				GetTransactionByIDErr:      errors.New(""),
				ChaincodeDefinitionRv:      &resourceconfig.MockChaincodeDefinition{EndorsementStr: "ESCC"},
				ExecuteResp:                expectedResponse,
			}
			es := NewEndorserServer(
				func(string, string, *rwset.TxPvtReadWriteSet) error { return nil },
				support,
			)
			t.Parallel()
			args := [][]byte{[]byte("args")}
			signedProp := getSignedPropWithCHIdAndArgs(tt.chainID, tt.chaincodeName, "version", args, t)

			resp, err := es.ProcessProposal(context.Background(), signedProp)
			assert.NoError(t, err)
			assert.Equal(t, expectedResponse.Payload, resp.Response.Payload)

			if tt.simAcquired {
				m.AssertCalled(t, "GetTxSimulator", mock.Anything, mock.Anything)
			} else {
				m.AssertNotCalled(t, "GetTxSimulator", mock.Anything, mock.Anything)
			}
		})
	}
}

var signer msp.SigningIdentity

func TestMain(m *testing.M) {
	// setup the MSP manager so that we can sign/verify
	err := msptesttools.LoadMSPSetupForTesting()
	if err != nil {
		fmt.Printf("Could not initialize msp/signer, err %s", err)
		os.Exit(-1)
		return
	}
	signer, err = mgmt.GetLocalMSP().GetDefaultSigningIdentity()
	if err != nil {
		fmt.Printf("Could not initialize msp/signer")
		os.Exit(-1)
		return
	}

	retVal := m.Run()
	os.Exit(retVal)
}

//go:generate counterfeiter -o mocks/support.go --fake-name Support . support
type support interface {
	Support
}

func TestUserCDSSanitization(t *testing.T) {
	fakeSupport := &mocks.Support{}
	e := NewEndorserServer(nil, fakeSupport)

	userCDS := &pb.ChaincodeDeploymentSpec{
		ChaincodeSpec: &pb.ChaincodeSpec{
			ChaincodeId: &pb.ChaincodeID{
				Name:    "user-cc-name",
				Version: "user-cc-version",
				Path:    "user-cc-path",
			},
			Input: &pb.ChaincodeInput{
				Args: [][]byte{[]byte("foo"), []byte("bar")},
			},
			Type: pb.ChaincodeSpec_GOLANG,
		},
		CodePackage: []byte("user-code"),
	}

	fsCDS := &pb.ChaincodeDeploymentSpec{
		ChaincodeSpec: &pb.ChaincodeSpec{
			ChaincodeId: &pb.ChaincodeID{
				Name:    "fs-cc-name",
				Version: "fs-cc-version",
				Path:    "fs-cc-path",
			},
			Type: pb.ChaincodeSpec_GOLANG,
		},
		CodePackage: []byte("fs-code"),
	}

	fakeSupport.GetChaincodeDeploymentSpecFSReturns(fsCDS, nil)

	sanitizedCDS, err := e.(*Endorser).SanitizeUserCDS(userCDS)
	assert.NoError(t, err)
	assert.Nil(t, sanitizedCDS.CodePackage)
	assert.True(t, proto.Equal(userCDS.ChaincodeSpec.Input, sanitizedCDS.ChaincodeSpec.Input))
	assert.True(t, proto.Equal(fsCDS.ChaincodeSpec.ChaincodeId, sanitizedCDS.ChaincodeSpec.ChaincodeId))

	t.Run("BadPath", func(t *testing.T) {
		fakeSupport.GetChaincodeDeploymentSpecFSReturns(nil, fmt.Errorf("fake-error"))
		_, err := e.(*Endorser).SanitizeUserCDS(userCDS)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "fake-error")
	})
}
