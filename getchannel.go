package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type ChannelResponse struct {
	Outputs map[string]*ChannelResponseOutput `json:"outputs"`
}

type ChannelResponseOutput struct {
	Capacity int64 `json:"capacity"`
	NodeLeft struct {
		Alias     string `json:"alias"`
		PublicKey string `json:"public_key"`
	} `json:"node_left"`
	NodeRight struct {
		Alias     string `json:"alias"`
		PublicKey string `json:"public_key"`
	} `json:"node_right"`
}

type Channel struct {
	TxID     string
	NodeA    string
	NodeB    string
	Capacity int64
	IsMulti  bool
}

var emptyChannelData = fmt.Errorf("empty channel data")

func getChannel(txid string, ch *Channel) error {
	w, err := http.Get("https://mempool.space/api/v1/lightning/channels/txids?txId[]=" + txid)
	if err != nil {
		return err
	}
	defer w.Body.Close()
	wb, err := io.ReadAll(w.Body)
	if err != nil {
		return err
	}

	var res []ChannelResponse
	err = json.Unmarshal(wb, &res)
	if err != nil {
		return err
	}

	var output *ChannelResponseOutput
	for _, o := range res[0].Outputs {
		output = o
		break
	}
	if output == nil {
		return emptyChannelData
	}

	ch.Capacity = output.Capacity
	ch.NodeA = output.NodeLeft.Alias
	if ch.NodeA == "" {
		ch.NodeA = output.NodeLeft.PublicKey[0:6]
	}
	ch.NodeB = output.NodeRight.Alias
	if ch.NodeB == "" {
		ch.NodeB = output.NodeRight.PublicKey[0:6]
	}

	return nil
}
