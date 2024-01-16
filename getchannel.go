package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type ChannelResponse struct {
	Outputs struct {
		First  *ChannelResponseOutput `json:"0"`
		Second *ChannelResponseOutput `json:"1"`
		Third  *ChannelResponseOutput `json:"2"`
	} `json:"outputs"`
}

type ChannelResponseOutput struct {
	Capacity int64 `json:"capacity"`
	NodeLeft struct {
		Alias string `json:"alias"`
	} `json:"node_left"`
	NodeRight struct {
		Alias string `json:"alias"`
	} `json:"node_right"`
}

type Channel struct {
	NodeA    string
	NodeB    string
	Capacity int64
}

func getChannel(txid string) (ch Channel, err error) {
	w, err := http.Get("https://mempool.space/api/v1/lightning/channels/txids?txId[]=" + txid)
	if err != nil {
		return ch, err
	}
	defer w.Body.Close()

	var res []ChannelResponse
	err = json.NewDecoder(w.Body).Decode(&res)
	if err != nil {
		return ch, err
	}

	output := res[0].Outputs.First
	if output == nil {
		output = res[0].Outputs.Second
		if output == nil {
			output = res[0].Outputs.Third
			if output == nil {
				return ch, fmt.Errorf("channel output is not in the first 3, so we gave up because the mempool.space api is crazy")
			}
		}
	}

	ch.Capacity = output.Capacity
	ch.NodeA = output.NodeLeft.Alias
	ch.NodeB = output.NodeRight.Alias
	return ch, nil
}
