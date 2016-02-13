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
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"camlistore.org/pkg/magic"

	"gopkg.in/errgo.v1"
	"gopkg.in/mqtt.v0"

	"github.com/spf13/cobra"
	"github.com/streadway/amqp"
)

func main() {
	server := os.Getenv("AMQP_URL")
	if server == "" {
		server = os.ExpandEnv("amqp://$AMQP_USER:$AMQP_PASSWORD@192.168.1.3:5672")
	}
	timeout := 5 * time.Second
	clientID, _ := os.Hostname()
	queue := "scanner"
	mainCmd := &cobra.Command{
		Use: "amqpc",
	}
	p := mainCmd.PersistentFlags()
	p.StringVarP(&server, "server", "S", server, "server address")
	p.DurationVarP(&timeout, "timeout", "", timeout, "timeout for commands")
	p.StringVarP(&clientID, "id", "", clientID, "client ID")
	p.StringVarP(&queue, "queue", "q", queue, "queue name to publish")

	appID := queue
	var noCompress bool
	pubCmd := &cobra.Command{
		Use:     "pub",
		Aliases: []string{"publish", "send", "write"},
		Run: func(_ *cobra.Command, args []string) {
			c, err := newClient(server, queue)
			if err != nil {
				log.Fatal(err)
			}
			defer c.Close()
			if err = c.Confirm(false); err != nil {
				log.Fatal(err)
			}
			confirms := c.NotifyPublish(make(chan amqp.Confirmation, 1))
			returns := c.NotifyReturn(make(chan amqp.Return, 1))

			var sendCount int
			tbl := make(map[string]interface{}, 1)
			for _, arg := range args {
				for k := range tbl {
					delete(tbl, k)
				}
				var r io.ReadCloser
				mimeType, contentEncoding := "text/plain", ""
				if strings.HasPrefix(arg, "@") {
					arg = arg[1:]
					if arg == "-" {
						r = os.Stdin
					} else if fh, err := os.Open(arg); err != nil {
						log.Fatal(err)
					} else {
						if noCompress {
							r = fh
						} else {
							pr, pw := io.Pipe()
							go func() {
								defer fh.Close()
								gw := gzip.NewWriter(pw)
								if _, err := io.Copy(gw, fh); err != nil {
									pw.CloseWithError(err)
									return
								}
								pw.CloseWithError(gw.Close())
							}()
							r = pr
							contentEncoding = "application/gzip"
						}
						tbl["FileName"] = arg
						if err := amqp.Table(tbl).Validate(); err != nil {
							log.Fatal(err)
						}
						mimeType = mime.TypeByExtension(filepath.Ext(arg))
					}
				} else {
					r = ioutil.NopCloser(strings.NewReader(arg))
				}
				b, err := ioutil.ReadAll(&io.LimitedReader{R: r, N: 256 << 20})
				r.Close()
				if err != nil {
					log.Fatal(err)
				}
				if mimeType == "" {
					if mimeType = magic.MIMEType(b); mimeType == "" {
						mimeType = "application/octet-stream"
					}
				}
				if err := c.Publish("", c.Queue.Name, false, false,
					amqp.Publishing{
						Headers:         tbl,
						DeliveryMode:    amqp.Persistent,
						ContentType:     mimeType,
						ContentEncoding: contentEncoding,
						AppId:           appID,
						Body:            b,
					},
				); err != nil {
					log.Fatalf("Publish: %v", err)
				}
				log.Printf("Sent %q", arg)
				sendCount++
			}

		Loop:
			for i := 0; i < sendCount; {
				select {
				case c, ok := <-confirms:
					if !ok {
						break Loop
					}
					if !c.Ack {
						log.Printf("couldn't deliver %d", c.DeliveryTag)
					} else {
						log.Printf("Delivered %d.", c.DeliveryTag)
						i++
					}
				case r, ok := <-returns:
					if !ok {
						break Loop
					}
					log.Printf("RETURN: %#v", r)
					i++
				}
			}
		},
	}
	f := pubCmd.Flags()
	f.StringVarP(&appID, "app-id", "", appID, "appID")
	f.BoolVarP(&noCompress, "no-compress", "", noCompress, "disable file data compression (for slow devices)")

	subCmd := &cobra.Command{
		Use:     "sub",
		Aliases: []string{"subscribe", "recv", "receive", "read"},
		Run: func(_ *cobra.Command, args []string) {
			c, err := newClient(server, queue)
			if err != nil {
				log.Fatal(err)
			}
			defer c.Close()

			d, err := c.Consume(c.Queue.Name, clientID, false, false, false, false, nil)
			if err != nil {
				log.Fatal(err)
			}
			tempDir, err := ioutil.TempDir("", "amqpc-")
			if err != nil {
				log.Fatal(err)
			}
			defer os.RemoveAll(tempDir)
			var i uint64
			for msg := range d {
				i++
				log.Printf("Received %s with %q from %s@%s.",
					msg.MessageId, msg.Headers, msg.UserId, msg.AppId)
				fn := msg.Headers["FileName"].(string)
				if fn == "" {
					var ext string
					if exts, err := mime.ExtensionsByType(msg.ContentType); err != nil {
						log.Printf("Extension for %q: %v", msg.ContentType)
					} else if len(ext) > 0 {
						ext = exts[0]
					}
					fn = fmt.Sprintf("%09d%s", i, ext)
				}
				fn = filepath.Join(tempDir, fn)
				log.Printf("Writing data to %q.", fn)
				r := ioutil.NopCloser(bytes.NewReader(msg.Body))
				if msg.ContentEncoding == "application/gzip" {
					if r, err = gzip.NewReader(r); err != nil {
						msg.Nack(false, true)
						log.Fatal(err)
					}
				}
				fh, err := os.Create(fn)
				if err == nil {
					_, err = io.Copy(fh, r)
				}
				if closeErr := fh.Close(); closeErr != nil && err == nil {
					err = closeErr
				}
				if err != nil {
					msg.Nack(false, true)
					os.Remove(fn)
					log.Fatal(err)
				}

				cmd := exec.Command(args[0], append(args[1:], fn)...)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				log.Printf("Calling %q", cmd.Args)
				if err := cmd.Run(); err != nil {
					msg.Nack(false, true)
					os.Remove(fn)
					log.Printf("%q: %v", cmd.Args, err)
					continue
				}
				os.Remove(fn)
				if err := msg.Ack(false); err != nil {
					log.Printf("cannot ACK %q: %v", msg, err)
				}
			}
		},
	}

	mainCmd.AddCommand(pubCmd, subCmd)
	mainCmd.Execute()
}

var msgHandler = mqtt.MessageHandler(func(client *mqtt.Client, msg mqtt.Message) {
	log.Printf("got message from %q (%v): %q", msg.Topic(), msg.MessageID(), msg.Payload())
})

type amqpClient struct {
	amqp.Queue
	*amqp.Channel
	*amqp.Connection
}

func (c *amqpClient) Close() error {
	var err error
	if c.Channel != nil {
		err = c.Channel.Close()
		c.Channel = nil
	}
	if c.Connection != nil {
		if closeErr := c.Connection.Close(); closeErr != nil && err == nil {
			err = nil
		}
		c.Connection = nil
	}
	return err
}

func newClient(server, queue string) (*amqpClient, error) {
	c := &amqpClient{}
	// Connections start with amqp.Dial() typically from a command line argument
	// or environment variable.
	var err error
	if c.Connection, err = amqp.Dial(server); err != nil {
		return nil, errgo.Notef(err, "url=%q", server)
	}

	// Most operations happen on a channel.  If any error is returned on a
	// channel, the channel will no longer be valid, throw it away and try with
	// a different channel.  If you use many channels, it's useful for the
	// server to
	if c.Channel, err = c.Connection.Channel(); err != nil {
		c.Close()
		return nil, errgo.Notef(err, "Channel")
	}

	// Declare your topology here, if it doesn't exist, it will be created, if
	// it existed already and is not what you expect, then that's considered an
	// error.
	if err = c.Channel.Qos(1, 0, false); err != nil {
		c.Close()
		return nil, errgo.Notef(err, "Qos")
	}

	// Use your connection on this topology with either Publish or Consume, or
	// inspect your queues with QueueInspect.  It's unwise to mix Publish and
	// Consume to let TCP do its job well.
	if c.Queue, err = c.Channel.QueueDeclare(queue, true, false, false, false, nil); err != nil {
		c.Close()
		return nil, errgo.Notef(err, "QueueDeclare")
	}

	return c, nil
}
