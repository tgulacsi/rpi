// Copyright 2016 Tamás Gulácsi
//
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

package main

import (
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"gopkg.in/errgo.v1"

	"github.com/eclipse/paho.mqtt.golang"
	"github.com/spf13/cobra"
)

var ErrTimeout = errgo.Newf("timeout")

// http://www.eclipse.org/paho/clients/golang/

func main() {
	server := "tcp://192.168.1.3:1883"
	topic := "topic"
	timeout := 5 * time.Second
	clientID, _ := os.Hostname()
	mainCmd := &cobra.Command{
		Use: "mqttc",
	}
	p := mainCmd.PersistentFlags()
	p.StringVarP(&server, "server", "S", server, "server address")
	p.DurationVarP(&timeout, "timeout", "", timeout, "timeout for commands")
	p.StringVarP(&clientID, "id", "", clientID, "client ID")

	store := "mqtt-store"
	qos := 1
	pubCmd := &cobra.Command{
		Use:     "pub",
		Aliases: []string{"publish", "send", "write"},
		Run: func(_ *cobra.Command, args []string) {
			client, err := newClient(server, clientID, store, timeout)
			if err != nil {
				log.Fatal(err)
			}
			defer client.Disconnect(uint(time.Second / time.Millisecond))

			for _, arg := range args {
				var r io.ReadCloser
				if strings.HasPrefix(arg, "@") {
					arg = arg[1:]
					if arg == "-" {
						r = os.Stdin
					} else if fh, err := os.Open(arg); err != nil {
						log.Fatal(err)
					} else {
						r = fh
					}
				} else {
					r = ioutil.NopCloser(strings.NewReader(arg))
				}
				b, err := ioutil.ReadAll(&io.LimitedReader{R: r, N: 256 << 20})
				r.Close()
				if err != nil {
					log.Fatal(err)
				}
				pt := client.Publish(topic, uint8(qos), true, b)
				if !pt.WaitTimeout(timeout) || pt.Error() != nil {
					if err := pt.Error(); err != nil {
						log.Fatal(err)
					}
					log.Fatalf("publish timeout")
				}
				log.Printf("Sent %q: %q", arg, pt.(*mqtt.PublishToken).MessageID())
			}
		},
	}
	f := pubCmd.Flags()
	p.StringVarP(&topic, "topic", "t", topic, "topic to publish")
	f.StringVarP(&store, "store", "", store, "path for mqtt store")
	f.IntVarP(&qos, "qos", "q", qos, "Quality of Service (0, 1 or 2)")

	subCmd := &cobra.Command{
		Use:     "sub",
		Aliases: []string{"subscribe", "recv", "receive", "read"},
		Run: func(_ *cobra.Command, args []string) {
			if len(args) > 0 {
				topic = args[0]
			}
			client, err := newClient(server, clientID, store, timeout)
			if err != nil {
				log.Fatal(err)
			}
			defer client.Disconnect(uint(time.Second / time.Millisecond))

			if st := client.Subscribe(topic, uint8(qos), msgHandler); !st.WaitTimeout(timeout) || st.Error() != nil {
				if err := st.Error(); err != nil {
					log.Fatal(err)
				}
				log.Fatal(errgo.WithCausef(nil, ErrTimeout, "subscribe"))
			}
			time.Sleep(3 * time.Second)
		},
	}

	mainCmd.AddCommand(pubCmd, subCmd)
	mainCmd.Execute()
}

var msgHandler = mqtt.MessageHandler(func(client *mqtt.Client, msg mqtt.Message) {
	log.Printf("got message from %q (%v): %q", msg.Topic(), msg.MessageID(), msg.Payload())
})

func newClient(server, clientID, store string, timeout time.Duration) (*mqtt.Client, error) {
	opts := mqtt.NewClientOptions().
		AddBroker(server).
		SetAutoReconnect(true).
		SetKeepAlive(2 * time.Second).
		SetPingTimeout(1 * time.Second).
		SetMaxReconnectInterval(1 * time.Minute).
		SetOrderMatters(false)
	if clientID == "" {
		if hn, err := os.Hostname(); err == nil {
			clientID = hn
		}
	}
	if clientID != "" {
		opts.SetClientID(clientID)
	}
	if store != "" {
		opts.SetStore(mqtt.NewFileStore(store))
	}
	client := mqtt.NewClient(opts)
	if ct := client.Connect(); !ct.WaitTimeout(timeout) || ct.Error() != nil {
		if err := ct.Error(); err != nil {
			return nil, err
		}
		return nil, errgo.WithCausef(nil, ErrTimeout, "connection")
	}
	return client, nil
}
