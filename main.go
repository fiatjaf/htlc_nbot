package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/kelseyhightower/envconfig"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
	"github.com/rs/zerolog"
)

const (
	RELAY_URL = "wss://fiatjaf.nostr1.com"
)

var (
	s   Settings
	log = zerolog.New(os.Stderr).Output(zerolog.ConsoleWriter{Out: os.Stdout}).With().Timestamp().Logger()
)

type Settings struct {
	SecretKey string `envconfig:"SECRET_KEY"`
}

func main() {
	if err := envconfig.Process("", &s); err != nil {
		log.Fatal().Err(err).Msg("failed to read from env")
		return
	}

	// fetch last blocks
	lastBlockInspected := 0
	if b, err := os.ReadFile("_last_block"); os.IsNotExist(err) {
		lastBlockInspected = 770000
	} else if err != nil {
		log.Fatal().Err(err).Msg("something is wrong with _last_block")
		return
	} else {
		lastBlockInspected, err = strconv.Atoi(strings.TrimSpace(string(b)))
		if err != nil {
			log.Fatal().Str("contents", string(b)).Err(err).Msg("something is wrong with _last_block")
			return
		}
	}

	for height := lastBlockInspected; ; height++ {
		os.WriteFile("_last_block", []byte(strconv.Itoa(height)), 0644)

		when, htlcs, err := inspectBlock(height)
		if err != nil {
			log.Fatal().Err(err).Int("height", height).Msg("error inspecting block")
			return
		}
		for _, htlc := range htlcs {
			if htlc.Fee < htlc.Amount/2 {
				continue
			}

			// publish nostr event
			event := nostr.Event{
				CreatedAt: when,
				Kind:      1,
				Content:   fmt.Sprintf("an #htlc costed %s sats to redeem but it was only worth %s sats in channel between '%s' and '%s': https://mempool.space/tx/%s;", comma(htlc.Fee), comma(htlc.Amount), htlc.Channel.NodeA, htlc.Channel.NodeB, htlc.TxID),
				Tags: nostr.Tags{
					nostr.Tag{"t", "htlc"},
				},
			}
			event.Sign(s.SecretKey)

			fmt.Println(event)

			relay, err := nostr.RelayConnect(context.Background(), RELAY_URL)
			if err != nil {
				log.Fatal().Err(err).Msg("failed to connect")
				return
			}

			if _, err := relay.Publish(context.Background(), event); err != nil {
				log.Fatal().Err(err).Msg("failed to publish")
				return
			}

			nevent, _ := nip19.EncodeEvent(event.ID, []string{RELAY_URL}, "")
			fmt.Println("https://njump.me/" + nevent)
		}
	}
}
