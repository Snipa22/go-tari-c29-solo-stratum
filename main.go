package main

import (
	"crypto/rand"
	"crypto/tls"
	"flag"
	"fmt"
	"net"

	"github.com/Snipa22/core-go-lib/helpers"
	core "github.com/Snipa22/core-go-lib/milieu"
	"github.com/Snipa22/go-tari-grpc-lib/v3/nodeGRPC"
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
	"github.com/snipa22/go-tari-c29-solo-stratum/subsystems/blockTemplateCache"
	"github.com/snipa22/go-tari-c29-solo-stratum/subsystems/config"
	"github.com/snipa22/go-tari-c29-solo-stratum/subsystems/messages"
	"github.com/snipa22/go-tari-c29-solo-stratum/subsystems/poolStratum"
	"github.com/snipa22/go-tari-c29-solo-stratum/subsystems/sslCert"
	"github.com/snipa22/go-tari-c29-solo-stratum/subsystems/tipDataCache"
)

// This needs to complete several steps
// 0. Initialize Global Milieu state
// 1. Bootstrap its config from the global instance
// 2. Spin up the verification goroutines/threads
// 3. Spin up the ports to listen on
// 4. Notify central backend that we're online.

func main() {
	// Build Milieu
	sentryURI := helpers.GetEnv("SENTRY_SERVER", "")
	milieu, err := core.NewMilieu(nil, nil, &sentryURI)
	if err != nil {
		milieu.CaptureException(err)
		milieu.Fatal(err.Error())
	}
	milieu.SetLogLevel(logrus.DebugLevel)
	// Milieu initialized

	// Load config flags
	nodeGRPCAddressPtr := flag.String("node-grpc-address", "node-pool.tari.jagtech.io:18102", "Tari base_node GRPC address")
	poolWalletAddressPtr := flag.String("pool-wallet-address", "1215dapiKwqGxk9TAjELMf9gnH6iKM5B9gLbMBvtDSVATRtnBsKDN8bfxGECaPC1wwA8AwRLnq1Ycg28Qx71uW8pABi", "Tari address that funds should be sent to")
	nextnetPtr := flag.Bool("nextnet", false, "Enable nextnet, changing the address verification scheme")
	debugEnabledPtr := flag.Bool("debug-enabled", false, "Enable Debug Logging")
	poolPort := flag.Int("pool-port", 3111, "Pool port for listening on")
	startingDiff := flag.Uint64("starting-diff", 2, "Set the starting difficulty for the port")
	sslEnabledPtr := flag.Bool("ssl-enabled", false, "Enable SSL")
	minDiff := flag.Uint64("min-diff", 1, "Set the minimum difficulty for the port")
	maxDiff := flag.Uint64("max-diff", 1000000000000, "Set the maximum difficulty for the port")
	flag.Parse()

	nodeGRPC.InitNodeGRPC(*nodeGRPCAddressPtr)

	config.StartingDifficulty = *startingDiff
	config.MinimumDifficulty = *minDiff
	config.MaxDifficulty = *maxDiff
	config.InitShareCount()

	if *nextnetPtr {
		config.AllowedAddressPrefixes = []string{"32", "34"}
	}

	// Perform initial data syncs
	tipDataCache.UpdateTipData(milieu)
	blockTemplateCache.UpdateBlockTemplateCache(milieu)

	// Build the cron spinner
	c := cron.New(cron.WithSeconds())

	config.SystemCrons.CronMaster = c

	config.PoolPayoutAddress = *poolWalletAddressPtr

	milieu.SetLogLevel(logrus.InfoLevel)
	if *debugEnabledPtr {
		milieu.SetLogLevel(logrus.DebugLevel)
	}

	// Add tasks to cron
	_, _ = c.AddFunc("* * * * * *", func() {
		tipDataCache.UpdateTipData(milieu)
	})

	_, _ = c.AddFunc("* * * * * *", func() {
		blockTemplateCache.UpdateBlockTemplateCache(milieu)
	})

	_, _ = c.AddFunc("*/30 * * * * *", func() {
		diff := config.MinerDiff.Get()
		diffText := "0 H/s"
		if diff != 0 {
			diff = diff / 30
		}
		if diff < 1000000000000000 {
			diffText = fmt.Sprintf("%.3f TH/s", float64(diff)/1000000000000)
		}
		if diff < 1000000000000 {
			diffText = fmt.Sprintf("%.3f GH/s", float64(diff)/1000000000)
		}
		if diff < 1000000000 {
			diffText = fmt.Sprintf("%.3f MH/s", float64(diff)/1000000)
		}
		if diff < 1000000 {
			diffText = fmt.Sprintf("%.3f KH/s", float64(diff)/1000)
		}
		if diff < 1000 {
			diffText = fmt.Sprintf("%d H/s", diff)
		}
		milieu.Info(fmt.Sprintf("%v/%v/%v/%v Valid/Trusted/Invalid/Total shares in last 30s - %v",
			config.ShareCount.Get("valid"),
			config.ShareCount.Get("trusted"),
			config.ShareCount.Get("invalid"),
			config.ShareCount.Get("total"),
			diffText))

	})

	// Start Verification
	// TODO: Start the verifier
	inboundWork := make(chan messages.HashToVerify, 1000)
	// Verifier routine initalized

	// Setup the GRPC ports for client connections
	cert, err := sslCert.GetCert()
	if err != nil {
		milieu.CaptureException(err)
		milieu.Fatal(err.Error())
	}
	tlscfg := tls.Config{Certificates: []tls.Certificate{cert}}
	tlscfg.Rand = rand.Reader
	go func(port int, ssl bool) {
		listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%v", port))
		if err != nil {
			milieu.CaptureException(err)
			milieu.Fatal(err.Error())
		}
		for {
			conn, err := listener.Accept()
			if err != nil {
				break
			}
			if ssl {
				conn = tls.Server(conn, &tlscfg)
			}
			defer conn.Close()
			go poolStratum.ClientConn(inboundWork, milieu)(conn)
		}
	}(*poolPort, *sslEnabledPtr)

	// Start all cron tasks
	c.Start()
	milieu.Info(fmt.Sprintf("Pool up and running on port %v, ready for connections", *poolPort))
	for {
		select {}
	}
}
