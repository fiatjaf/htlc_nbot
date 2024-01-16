package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type ChannelResponse struct {
	Outputs struct {
		First   *ChannelResponseOutput `json:"0"`
		Second  *ChannelResponseOutput `json:"1"`
		Third   *ChannelResponseOutput `json:"2"`
		Fourth  *ChannelResponseOutput `json:"3"`
		Fifth   *ChannelResponseOutput `json:"4"`
		Sixth   *ChannelResponseOutput `json:"5"`
		Seventh *ChannelResponseOutput `json:"6"`
	} `json:"outputs"`
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
				output = res[0].Outputs.Fourth
				if output == nil {
					output = res[0].Outputs.Fifth
					if output == nil {
						output = res[0].Outputs.Sixth
						if output == nil {
							output = res[0].Outputs.Seventh
							if output == nil {
								return ch, fmt.Errorf("channel output is not in the first 7, so we gave up because the mempool.space api is crazy")
							}
						}
					}
				}
			}
		}
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
	return ch, nil
}
