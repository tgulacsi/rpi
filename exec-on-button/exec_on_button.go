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

// Execute the given program on button push.
//
// Button:
//   * execute the script on light (more than 500ms but less then 3s) push,
//   * kill the running script after a more the 5s push.
//
// LED:
//   * just blink it rarely when idle,
//   * on/off 1/s while script is executing,
//   * on/off randomly if the script exited with error,
//
// Hardware design: http://raspi.tv/2013/rpi-gpio-basics-6-using-inputs-and-outputs-together-with-rpi-gpio-pull-ups-and-pull-downs
package main

import (
	"flag"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/stianeikeland/go-rpio"
	"golang.org/x/net/context"
)

const (
	eventLoopDuration = 100 * time.Millisecond

	LongPress  = 5 * time.Second
	ShortPress = 2 * time.Second
	MinPress   = 100 * time.Millisecond
)

var (
	IdleTimes       = times{On: 100 * time.Millisecond, Off: 2 * time.Second}
	InProgressTimes = times{On: 750 * time.Millisecond, Off: 500 * time.Millisecond}
	ErrorTimes      = times{On: 100 * time.Millisecond, Off: 150 * time.Millisecond}
)

type ButtonEvent uint8

const (
	StartEvent = ButtonEvent(iota)
	StopEvent
)

var debug = func(pattern string, args ...interface{}) {}

func main() {
	flagButtonPin := flag.Int("button", 25, "button pin")
	flagLEDPin := flag.Int("led", 24, "LED pin")
	flagHTTP := flag.String("http", "0.0.0.0:1234", "HTTP listen address")
	flagVerbose := flag.Bool("v", false, "verbose logging")
	flag.Parse()

	if err := rpio.Open(); err != nil {
		log.Fatal(err)
	}
	defer rpio.Close()
	if *flagVerbose {
		debug = log.Printf
	}

	button := rpio.Pin(*flagButtonPin)
	button.Input()

	led := rpio.Pin(*flagLEDPin)
	led.Output()

	ledCh := make(chan times, 1)
	defer close(ledCh)
	ledCh <- IdleTimes
	go blink(led, ledCh)

	errCh := make(chan error, 1)
	var ctx context.Context
	var cancel func()
	events := make(chan ButtonEvent, 1)
	if *flagHTTP != "" {
		http.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) { events <- StartEvent })
		http.HandleFunc("/stop", func(w http.ResponseWriter, r *http.Request) { events <- StopEvent })
		log.Printf("Start listening on %q...", *flagHTTP)
		go http.ListenAndServe(*flagHTTP, nil)
	} else {
		log.Println("Started.")
	}
	go getEvents(events, button)
	for {
		select {
		case event := <-events:
			switch event {
			case StopEvent:
				if cancel == nil { // nothing in progress
					log.Printf("received STOP, but nothing is in progress")
					ledCh <- IdleTimes
					continue
				}
				log.Printf("STOPping...")
				cancel()
				cancel = nil
				ledCh <- IdleTimes
			case StartEvent:
				if cancel != nil { // action in progress
					log.Printf("received START, but already running!")
					continue
				}
				ledCh <- InProgressTimes
				ctx, cancel = context.WithCancel(context.Background())
				go run(ctx, errCh, exec.Command(flag.Args()[0], flag.Args()[1:]...))
				log.Printf("STARTing %v", flag.Args())
				select {
				case err := <-errCh:
					log.Printf("error starting %v: %v", flag.Args(), err)
					ledCh <- ErrorTimes
				default:
					ledCh <- InProgressTimes
				}
			}
		case err := <-errCh:
			if cancel != nil {
				cancel()
			}
			cancel = nil
			if err == nil {
				log.Printf("Command successfully finished.")
				ledCh <- IdleTimes
			} else {
				log.Printf("ERROR running %v: %v", flag.Args(), err)
				ledCh <- ErrorTimes
			}
		}
	}
}

func blink(led rpio.Pin, dCh <-chan times) {
	state := rpio.Low
	led.Write(state)
	defer led.Write(rpio.Low)
	d := IdleTimes
	for {
		select {
		case d = <-dCh:
		case <-time.After(d.Duration(state == rpio.High)):
		}
		state = rpio.State((state + 1) % 2)
		led.Write(state)
	}
}

type times struct {
	On, Off time.Duration
}

func (t times) Duration(on bool) time.Duration {
	d := t.On
	if !on {
		d = t.Off
	}
	if d < 0 {
		d = time.Duration(float32(-d) * (0.5 + rand.Float32()/2))
	}
	//debug("duration(%t)=%s", on, d)
	return d
}

// run the given command, within the given context.
// On cancel, kill the children.
func run(ctx context.Context, errCh chan<- error, cmd *exec.Cmd) {
	select {
	case <-ctx.Done():
		errCh <- ctx.Err()
		return
	default:
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		errCh <- err
		return
	}
	finish := make(chan error, 1)
	go func() { finish <- cmd.Wait() }()
	select {
	case <-ctx.Done():
		cmd.Process.Kill()
		errCh <- ctx.Err()
	case err := <-finish:
		errCh <- err
	}
}

// getEventsCh returns a channel which gives the events from the button presses.
func getEventsCh(button rpio.Pin) <-chan ButtonEvent {
	buttonCh := make(chan ButtonEvent)
	go getEvents(buttonCh, button)
	return buttonCh
}

func getEvents(buttonCh chan<- ButtonEvent, button rpio.Pin) {
	for press := range getButtonPresses(button) {
		if press < MinPress {
			continue
		}
		if press > LongPress {
			buttonCh <- StopEvent
		}
		if press < ShortPress {
			buttonCh <- StartEvent
		}
	}
}

// getButtonPresses returns a channel which gives the button hold durations.
func getButtonPresses(button rpio.Pin) <-chan time.Duration {
	ch := make(chan time.Duration)
	go func() {
		down := button.Read() == rpio.High
		var start time.Time
		if down {
			start = time.Now()
		}
		for now := range time.NewTicker(eventLoopDuration).C {
			act := button.Read() == rpio.High
			if down == act {
				continue
			}
			debug("old=%t act=%t", down, act)
			if down && !act {
				ch <- time.Since(start)
			} else if !down && act {
				start = now
			}
			down = act
		}
	}()
	return ch
}
