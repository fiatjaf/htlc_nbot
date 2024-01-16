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

var (
	s   Settings
	log = zerolog.New(os.Stderr).Output(zerolog.ConsoleWriter{Out: os.Stdout}).With().Timestamp().Logger()
)

type Settings struct {
	SecretKey string `envconfig:"SECRET_KEY"`
	RelayURL  string `envconfig:"RELAY_URL" default:"wss://fiatjaf.nostr1.com"`
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

	pubkey, _ := nostr.GetPublicKey(s.SecretKey)
	log.Info().Str("pubkey", pubkey).Str("relay", s.RelayURL).Msg("starting")

	for height := lastBlockInspected; ; height++ {
		os.WriteFile("_last_block", []byte(strconv.Itoa(height)), 0644)

		when, htlcs, err := inspectBlock(height)
		if err != nil {
			log.Fatal().Err(err).Int("height", height).Msg("error inspecting block")
			return
		}
		for _, htlc := range htlcs {
			if htlc.Amount > 9_000 {
				// for big htlcs we will accept non-negative closures as horrible
				if htlc.Fee < htlc.Amount*2/3 {
					continue
				}
			} else {
				// otherwise only negative closures
				if htlc.Fee < htlc.Amount {
					continue
				}
			}

			content := fmt.Sprintf(
				"an #htlc was worth %s sats, but it costed %s sats to redeem in ",
				comma(htlc.Amount), comma(htlc.Fee),
			)
			if htlc.Channel.NodeA != "" && htlc.Channel.NodeB != "" {
				content += fmt.Sprintf("channel between '%s' and a '%s': ",
					htlc.Channel.NodeA, htlc.Channel.NodeB)
			} else if htlc.Channel.NodeA != "" && htlc.Channel.NodeB == "" {
				content += fmt.Sprintf("channel from '%s': ", htlc.Channel.NodeA)
			} else if htlc.Channel.NodeB != "" && htlc.Channel.NodeA == "" {
				content += fmt.Sprintf("channel from '%s': ", htlc.Channel.NodeB)
			} else {
				content += fmt.Sprintf("a private channel from a mobile wallet: ")
			}
			content += fmt.Sprintf("https://mempool.space/tx/%s/", htlc.TxID)

			if htlc.Fee > htlc.Amount {
				content += fmt.Sprintf("\n\nsomeone lost their entire payment and paid %s sats more to the gods of lightning for the privilege", comma(htlc.Fee-htlc.Amount))
			} else if htlc.Fee == htlc.Amount {
				content += "so they basically sacrificed their channel in order to gain nothing"
			} else {
				content += fmt.Sprintf("\n\n so they probably lost a channel in order to recover just %s sats, gifting %d%% to the gods of lightning", comma(htlc.Amount-htlc.Fee), htlc.Fee*100/htlc.Amount)
			}

			// publish nostr event
			event := nostr.Event{
				CreatedAt: when,
				Kind:      1,
				Content:   content,
				Tags: nostr.Tags{
					nostr.Tag{"t", "htlc"},
				},
			}
			event.Sign(s.SecretKey)

			fmt.Println(event)

			relay, err := nostr.RelayConnect(context.Background(), s.RelayURL)
			if err != nil {
				log.Fatal().Err(err).Msg("failed to connect")
				return
			}

			if _, err := relay.Publish(context.Background(), event); err != nil {
				log.Fatal().Err(err).Msg("failed to publish")
				return
			}

			nevent, _ := nip19.EncodeEvent(event.ID, []string{s.RelayURL}, "")
			fmt.Println("https://njump.me/" + nevent)
		}
	}
}
