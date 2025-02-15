package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	ping "go_ping/pkg/go_ping"
)

func main() {
	var (
		count    = flag.Int("c", 0, "Number of packets to send")
		interval = flag.Duration("i", time.Second, "Interval between packets")
		timeout  = flag.Duration("t", time.Millisecond*100, "Timeout waiting for response")
		flood    = flag.Bool("f", false, "Flood ping (super-user only)")
	)

	flag.Parse()
	target := flag.Arg(0)
	intervalFlag := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "i" {
			intervalFlag = true
		}
	})

	if target == "" {
		fmt.Println("Error: target is required")
		os.Exit(1)
	}

	if os.Getuid() != 0 {
		fmt.Println("Error: ping requires root privileges")
		os.Exit(1)
	}

	if *flood && !intervalFlag {
		*interval = 0
	}

	pinger, err := ping.NewPinger(target)
	if err != nil {
		log.Fatalf("Error creating pinger: %v", err)
	}

	pinger.SetInterval(*interval)
	pinger.SetTimeout(*timeout)
	if *count > 0 {
		pinger.SetCount(*count)
	}

	// Handle Ctrl+C
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		pinger.Stop()
	}()

	if *flood {
		pinger.SetFloodMode(true)
		pinger.OnRecv = func(pkt *ping.Packet) {
			fmt.Print("\b")
		}
	} else {
		pinger.OnRecv = func(pkt *ping.Packet) {
			fmt.Printf("%d bytes from %s: icmp_seq=%d time=%v\n",
				pkt.NBytes, pkt.IPAddr, pkt.Seq, pkt.Rtt)
		}
	}

	pinger.OnFinish = func(stats *ping.Statistics) {
		fmt.Printf("\n--- %s ping statistics ---\n", stats.Addr)
		fmt.Printf("%d packets transmitted, %d received, %.0f%% packet loss\n",
			stats.PacketsSent, stats.PacketsRecv, stats.PacketLoss)
		fmt.Printf("rtt min/avg/max = %v/%v/%v\n",
			stats.MinRtt, stats.AvgRtt, stats.MaxRtt)
	}

	fmt.Printf("PING %s (%s):\n", pinger.Addr(), pinger.IPAddr().String())
	err = pinger.Run()
	if err != nil {
		log.Fatalf("Error pinging target: %v", err)
	}
}
