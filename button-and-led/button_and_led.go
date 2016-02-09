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
