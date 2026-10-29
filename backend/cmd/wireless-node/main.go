package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"pentagi/pkg/wireless/node"
)

func main() {
	var config node.Config
	var tokenFile, profilesFile string
	flag.StringVar(&config.URL, "server", "", "HTTPS PentAGI origin")
	flag.StringVar(&config.NodeID, "node", "", "enrolled node ID")
	flag.StringVar(&config.Interface, "interface", "", "dedicated Linux wireless interface")
	flag.StringVar(&config.Network, "network", "", "prepared ipvlan/macvlan Docker network")
	flag.StringVar(&config.StateFile, "state", "/var/lib/pentagi-wireless/state.json", "local recovery journal")
	flag.StringVar(&tokenFile, "token-file", "", "file containing the enrollment token")
	flag.StringVar(&profilesFile, "profiles", "", "JSON map of BSSID to saved NetworkManager connection UUID")
	flag.DurationVar(&config.ScanInterval, "scan-interval", 15*time.Second, "idle passive observation interval; zero disables it")
	flag.Parse()
	if runtime.GOOS != "linux" {
		log.Fatal("wireless nodes require Linux")
	}
	token, err := os.ReadFile(tokenFile)
	if err != nil {
		log.Fatal("cannot read node token file")
	}
	config.Token = strings.TrimSpace(string(token))
	if profilesFile != "" {
		raw, err := os.ReadFile(profilesFile)
		if err != nil {
			log.Fatal(err)
		}
		if err := json.Unmarshal(raw, &config.Profiles); err != nil {
			log.Fatal(err)
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	agent := node.Agent{Config: config}
	if err := agent.Serve(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
}
