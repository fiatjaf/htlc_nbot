build:
    CC=musl-gcc go build -ldflags="-s -w" -o ./htlc_nbot

deploy: build
    ssh root@turgot 'systemctl stop htlc_nbot'
    scp htlc_nbot turgot:htlc_nbot/htlc_nbot
    ssh root@turgot 'systemctl start htlc_nbot'
