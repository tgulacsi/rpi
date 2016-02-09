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

// Light (blink frequently) while the button is pressed.
//
// Hardware design: http://raspi.tv/2013/rpi-gpio-basics-6-using-inputs-and-outputs-together-with-rpi-gpio-pull-ups-and-pull-downs
package main

import (
	"log"
	"time"

	"github.com/stianeikeland/go-rpio"
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
