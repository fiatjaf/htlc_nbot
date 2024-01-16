package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/btcsuite/btcd/wire"
)

var esploras = []string{
	"https://mempool.space/api",
	"https://blockstream.info/api",
	"https://mempool.emzy.de/api",
}

var blockFetchFunctions = []func(hash string) ([]byte, error){
	blockFromBlockchainInfo,
	blockFromBlockchair,
	blockFromEsplora,
}
var current int

func getBlock(height int) (*wire.MsgBlock, error) {
	hash, err := getHash(height)
	if err != nil {
		return nil, fmt.Errorf("no hash for block %d", height)
	}

	errs := make([]error, 0, 3)
	for i := 0; i < len(blockFetchFunctions); i++ {
		current++
		fetchBlock := blockFetchFunctions[(i+current)%len(blockFetchFunctions)]
		rawBlock, err := fetchBlock(hash)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		block := &wire.MsgBlock{}
		if err := block.Deserialize(bytes.NewReader(rawBlock)); err != nil {
			errs = append(errs, err)
			continue
		}

		return block, nil
	}

	return nil, errors.Join(errs...)
}

func getHash(height int) (hash string, err error) {
	for _, endpoint := range esploras {
		w, errW := http.Get(fmt.Sprintf(endpoint+"/block-height/%d", height))
		if errW != nil {
			err = errW
			continue
		}
		defer w.Body.Close()

		if w.StatusCode >= 404 {
			continue
		}

		data, errW := io.ReadAll(w.Body)
		if errW != nil {
			err = errW
			continue
		}

		hash = strings.TrimSpace(string(data))

		if len(hash) > 64 {
			err = errors.New("got something that isn't a block hash: " + hash[:64])
			continue
		}

		return hash, nil
	}

	return "", err
}

func blockFromBlockchainInfo(hash string) ([]byte, error) {
	w, err := http.Get(fmt.Sprintf("https://blockchain.info/rawblock/%s?format=hex", hash))
	if err != nil {
		return nil, fmt.Errorf("failed to get raw block %s from blockchain.info: %s", hash, err.Error())
	}
	defer w.Body.Close()

	block, _ := io.ReadAll(w.Body)
	if len(block) < 100 {
		// block not available here yet
		return nil, nil
	}

	blockbytes, err := hex.DecodeString(string(block))
	if err != nil {
		return nil, fmt.Errorf("block from blockchain.info is invalid hex: %w", err)
	}

	return blockbytes, nil
}

func blockFromBlockchair(hash string) ([]byte, error) {
	url := "https://api.blockchair.com/bitcoin/raw/block/"
	w, err := http.Get(url + hash)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get raw block %s from blockchair.com: %s", hash, err.Error())
	}
	defer w.Body.Close()

	var data struct {
		Data map[string]struct {
			RawBlock string `json:"raw_block"`
		} `json:"data"`
	}
	err = json.NewDecoder(w.Body).Decode(&data)
	if err != nil {
		return nil, err
	}

	if bdata, ok := data.Data[hash]; ok {
		blockbytes, err := hex.DecodeString(bdata.RawBlock)
		if err != nil {
			return nil, fmt.Errorf("block from blockchair is invalid hex: %w", err)
		}

		return blockbytes, nil
	} else {
		// block not available here yet
		return nil, nil
	}
}

func blockFromEsplora(hash string) ([]byte, error) {
	var err error
	var block []byte

	for _, endpoint := range esploras {
		w, errW := http.Get(fmt.Sprintf(endpoint+"/block/%s/raw", hash))
		if errW != nil {
			err = errW
			continue
		}

		defer w.Body.Close()
		block, _ = io.ReadAll(w.Body)

		if len(block) < 200 {
			// block not available yet
			return nil, nil
		}

		break
	}

	return block, err
}
