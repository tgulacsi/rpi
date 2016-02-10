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
	"log"
	"time"

	"github.com/stianeikeland/go-rpio"
)

const (
	eventLoopDuration = 100 * time.Millisecond

	LongPress  = 5 * time.Second
	ShortPress = 2 * time.Second
	MinPress   = 500 * time.Millisecond
)

type ButtonEvent uint8

const (
	StartEvent = ButtonEvent(iota)
	StopEvent
)

func main() {
	if err := rpio.Open(); err != nil {
		log.Fatal(err)
	}
	defer rpio.Close()

	button := rpio.Pin(25)
	button.Input()

	led := rpio.Pin(24)
	led.Output()

	getEvents(button)

	ledCh := make(chan time.Duration)
	go func() {
		dCh := make(chan time.Duration)
		go func() {
			state := rpio.Low
			d := time.Second
			for {
				select {
				case d = <-dCh:
				case <-time.After(d):
				}
				state = rpio.State((state + 1) % 2)
				led.Write(state)
			}
		}()
		led.Write(rpio.Low)
		for d := range ledCh {
			if d < 0 {
				led.Write(rpio.Low)
				continue
			}
			led.Write(rpio.High)
			if d == 0 {
				continue
			}
			dCh <- d
		}
	}()
	defer close(ledCh)

	inCh := make(chan rpio.State)
	go func() {
		old := button.Read()
		inCh <- old
		for {
			act := button.Read()
			if act != old {
				old = act
				inCh <- act
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	ledCh <- time.Second
	for state := range inCh {
		if state == rpio.High {
			ledCh <- time.Duration(333 * time.Millisecond)
		} else {
			ledCh <- time.Duration(1 * time.Second)
		}
	}
}

// getEvents returns a channel which gives the events from the button presses.
func getEvents(button rpio.Pin) <-chan ButtonEvent {
	buttonCh := make(chan ButtonEvent)
	go func() {
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
	}()
	return buttonCh
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
