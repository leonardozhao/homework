package main

import (
	configtxtest "github.com/hyperledger/fabric/common/configtx/test"
	cb "github.com/hyperledger/fabric/protos/common"
	"fmt"
	"github.com/golang/protobuf/proto"
	"flag"
	"os"
)

func PrintBlockInfo() {
	block, err := configtxtest.MakeGenesisBlock(channelId)
	if err != nil {
		fmt.Println("Make genesis block failed!")
	}

	if block == nil || block.Data == nil || block.Data.Data == nil || len(block.Data.Data) == 0 {
		fmt.Println(fmt.Errorf("failed to retrieve channel id - block is empty"))
	}

	envelope := &cb.Envelope{}
	if err = proto.Unmarshal(block.Data.Data[0], envelope); err != nil {
		fmt.Println(fmt.Errorf("error reconstructing envelope(%s)", err))
	}
	payload := &cb.Payload{}
	if err = proto.Unmarshal(envelope.Payload, payload); err != nil {
		fmt.Println(fmt.Errorf("error reconstructing payload(%s)", err))
	}

	if payload.Header == nil {
		fmt.Println(fmt.Errorf("failed to retrieve channel id - payload header is empty"))
	}
	chdr, err := UnmarshalChannelHeader(payload.Header.ChannelHeader)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println("Channel Id: ", chdr.ChannelId)
	fmt.Println("Current block number: ", block.Header.Number)
	fmt.Println("Block timestamp: ", chdr.Timestamp)
	fmt.Println("Previous block hash is: ", block.Header.PreviousHash)
	dataHash := block.Header.DataHash
	fmt.Println("Block data hash is: ", dataHash)

	fmt.Println("Transaction ID: ", chdr.TxId)
	fmt.Println("Transaction type: ", cb.HeaderType_name[chdr.Type])

}

// UnmarshalChannelHeader returns a ChannelHeader from bytes
func UnmarshalChannelHeader(bytes []byte) (*cb.ChannelHeader, error) {
	chdr := &cb.ChannelHeader{}
	err := proto.Unmarshal(bytes, chdr)
	if err != nil {
		return nil, fmt.Errorf("UnmarshalChannelHeader failed, err %s", err)
	}

	return chdr, nil
}

var name, channelId string

func init() {
	flag.StringVar(&name, "name", "", "enter your name")
	flag.StringVar(&channelId, "channelId", "", "enter a channel id here")
}

func main() {
	flag.Parse()

	if name == "" || channelId == "" {
		flag.Usage()
		os.Exit(0)
	}
	fmt.Println("Welcome ", name)
	PrintBlockInfo()
}
