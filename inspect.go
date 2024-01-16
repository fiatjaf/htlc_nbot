package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/nbd-wtf/go-nostr"
)

type HTLC struct {
	TxID    string
	Amount  int64
	Fee     int64
	Channel Channel
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
			// go through their inputs and see if one has a script that looks like an HTLC
			for _, inp := range tx.TxIn {
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
					// it's an htlc! get the inputs to this transaction so we can calculate the fee
					htlc := HTLC{TxID: tx.TxHash().String()}
					for _i, _inp := range tx.TxIn {
						txid := _inp.PreviousOutPoint.Hash.String()
						prevTx, err := getTransaction(txid)
						if err != nil {
							return 0, nil, fmt.Errorf("failed to get transaction %s", txid)
						}

						inputValue := prevTx.Vout[_inp.PreviousOutPoint.Index].Value
						htlc.Fee += inputValue

						if _inp == inp {
							htlc.Amount = inputValue
							htlc.Channel, err = getChannel(prevTx.Vin[0].TXID)
							if err != nil {
								log.Warn().Err(err).Str("htlc", htlc.TxID).Str("funding", prevTx.Vin[0].TXID).
									Int("input", _i).Msg("error getting channel data, proceeding with nothing")
							}
						}
					}
					for _, _outp := range tx.TxOut {
						htlc.Fee -= _outp.Value
					}

					htlcs = append(htlcs, htlc)

					// then exit here (assume for simplicity that there can't be two HTLCs in the same tx)
					break
				}
			}
		}

		return nostr.Timestamp(block.Header.Timestamp.Unix()), htlcs, nil
	}
}
