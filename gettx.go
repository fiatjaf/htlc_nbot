package main

import (
	"encoding/json"
	"net/http"
)

type UTXOResponse struct {
	Amount *int64  `json:"amount"`
	Script *string `json:"script"`
}

type TxResponse struct {
	TXID string   `json:"txid"`
	Vout []TxVout `json:"vout"`
	Vin  []TxIn   `json:"vin"`
}

type TxIn struct {
	TXID string `json:"txid"`
}

type TxVout struct {
	ScriptPubKey string `json:"scriptPubKey"`
	Value        int64  `json:"value"`
}

func getTransaction(txid string) (tx TxResponse, err error) {
	// then try explorers
	for _, endpoint := range esploras {
		w, errW := http.Get(endpoint + "/tx/" + txid)
		if errW != nil {
			err = errW
			continue
		}
		defer w.Body.Close()

		errW = json.NewDecoder(w.Body).Decode(&tx)
		if errW != nil {
			err = errW
			continue
		}

		return tx, nil
	}

	return
}
