package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/nbd-wtf/go-nostr"
	"golang.org/x/exp/slices"
)

type HTLC struct {
	TxID       string
	Amount     int64
	Fee        int64
	Channel    Channel
	TotalHTLCs uint16
	Prevouts   []Prevout
}

type Prevout struct {
	wire.OutPoint
	IsHTLC bool
}

func (htlc HTLC) isPrintable() bool {
	if htlc.Amount > 9_000 {
		// for big htlcs we will accept non-negative closures as horrible
		return htlc.Fee >= htlc.Amount*2/3
	} else {
		// otherwise only negative closures
		return htlc.Fee >= htlc.Amount
	}
}

func inspectBlock(n int) (nostr.Timestamp, []HTLC, error) {
	log.Info().Int("n", n).Msg("inspecting block")
	htlcs := make([]HTLC, 0, 50)

	var block *wire.MsgBlock
	for {
		var err error
		block, err = getBlock(n)
		if err != nil && strings.HasPrefix(err.Error(), "no hash for block") {
			// wait 10 minutes and try again
			time.Sleep(10 * time.Minute)
			continue
		} else if err != nil {
			return 0, nil, fmt.Errorf("something went wrong when trying to fetch block: %w", err)
		}

		// inspect every transaction
		for _, tx := range block.Transactions {
			if !tx.HasWitness() {
				continue
			}

			// we'll add together all htlcs in this same tx into a single (in case it's a big sweep)
			htlc := HTLC{
				TxID:     tx.TxHash().String(),
				Prevouts: make([]Prevout, len(tx.TxIn)),
			}

			// go through their inputs and see if one has a script that looks like an HTLC
			for i, inp := range tx.TxIn {
				// add all inputs to this array
				htlc.Prevouts[i].OutPoint = inp.PreviousOutPoint
				// then later if we find out they are htlc we set that flag to true

				if len(inp.Witness) < 2 {
					continue
				}
				encodedScript := inp.Witness[len(inp.Witness)-1]
				script, err := txscript.DisasmString(encodedScript)
				if err != nil {
					continue
				}
				if strings.HasPrefix(script, "OP_DUP OP_HASH160") &&
					strings.HasPrefix(script[59:], "OP_EQUAL OP_IF OP_CHECKSIG OP_ELSE") &&
					strings.HasPrefix(script[161:], "OP_SWAP OP_SIZE 20 OP_EQUAL OP_NOTIF OP_DROP 2 OP_SWAP") {
					// it's an htlc!
					htlc.Prevouts[i].IsHTLC = true
					htlc.TotalHTLCs++
				}
			}

			if htlc.TotalHTLCs == 0 {
				continue
			}

			// if we got here it means we found at least one htlc
			prevTxs := make([]TxResponse, len(htlc.Prevouts))
			for i, prevout := range htlc.Prevouts {
				var prevTx TxResponse
				idx := slices.IndexFunc(htlc.Prevouts, func(po Prevout) bool { return po.Hash.IsEqual(&prevout.Hash) })
				if idx != i {
					// duplicate, reuse tx
					prevTx = prevTxs[idx]
				} else {
					// not duplicate, fetch
					tx, err := getTransaction(prevout.Hash.String())
					if err != nil {
						return 0, nil, fmt.Errorf("failed to get transaction %s", prevout.Hash.String())
					}
					prevTx = tx
					prevTxs[i] = prevTx // put it here so we'll reuse later
				}

				if len(prevTx.Vout) == 0 {
					panic("this should never happen")
				}

				inputValue := prevTx.Vout[prevout.Index].Value

				htlc.Fee += inputValue // we'll add all the inputs to the fee total, then later subtract the outputs
				if prevout.IsHTLC {
					htlc.Amount += inputValue // add all htlc amounts in a single sum in case of a batch sweep

					// set this channel funding txid here so we will fetch it later
					channelFundingTx := prevTx.Vin[0].TXID
					// unless we have already set
					if htlc.Channel.TxID == "" {
						htlc.Channel.TxID = channelFundingTx
					} else if htlc.Channel.TxID != channelFundingTx {
						// if we did and it was a different one, then we will unset it and mark the thing coming from "multiple channels"
						// (this probably never happens)
						htlc.Channel.TxID = ""
						htlc.Channel.IsMulti = true
					}
				}
			}

			// the remaining of inputs - outputs is the fee
			for _, _outp := range tx.TxOut {
				htlc.Fee -= _outp.Value
			}

			// as we have the full fee and amount now we can check if this is worth publishing
			if !htlc.isPrintable() {
				// otherwise stop here
				continue
			}

			// this htlc is printable, so we are ready to fetch channel metadata
			if !htlc.Channel.IsMulti {
				if err := getChannel(htlc.Channel.TxID, &htlc.Channel); err != nil {
					log.Warn().Err(err).Str("htlc", htlc.TxID).Str("funding", htlc.Channel.TxID).
						Msg("error getting channel data, proceeding with nothing")
				}
			}

			htlcs = append(htlcs, htlc)
		}

		return nostr.Timestamp(block.Header.Timestamp.Unix()), htlcs, nil
	}
}
