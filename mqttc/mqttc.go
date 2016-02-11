package main

import (
	"flag"
	"log"
	"os"
	"time"

	"github.com/eclipse/paho.mqtt.golang"
)

// http://www.eclipse.org/paho/clients/golang/

func main() {
	flagServer := flag.String("server", "tcp://192.168.1.3:1883", "server address")
	flagStore := flag.String("store", "mqtt-store", "path for mqtt store")
	flagQoS := flag.Int("qos", 1, "quality-of-service")
	flagTopic := flag.String("topic", "#", "topic")
	flagTimeout := flag.Duration("timeout", 5*time.Second, "timeout for connection/publish")
	flag.Parse()

	opts := mqtt.NewClientOptions().
		AddBroker(*flagServer).
		SetAutoReconnect(true).
		SetKeepAlive(2 * time.Second).
		SetPingTimeout(1 * time.Second).
		SetMaxReconnectInterval(1 * time.Minute).
		SetOrderMatters(false)
	if hn, err := os.Hostname(); err == nil {
		opts.SetClientID(hn)
	}
	if *flagStore != "" {
		opts.SetStore(mqtt.NewFileStore(*flagStore))
	}
	client := mqtt.NewClient(opts)
	if ct := client.Connect(); !ct.WaitTimeout(*flagTimeout) || ct.Error() != nil {
		if err := ct.Error(); err != nil {
			log.Fatal(err)
		}
		log.Fatalf("timeout connection")
	}
	defer client.Disconnect(uint(time.Second / time.Millisecond))

	pt := client.Publish(*flagTopic, uint8(*flagQoS), true, []byte("aa"))
	if !pt.WaitTimeout(*flagTimeout) || pt.Error() != nil {
		if err := pt.Error(); err != nil {
			log.Fatal(err)
		}
		log.Fatalf("publish timeout")
	}
}
