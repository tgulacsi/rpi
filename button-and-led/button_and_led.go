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

	old := button.Read()
	for {
		act := button.Read()
		if act == old {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if act == rpio.High {
			log.Printf("Port 25 is HIGH - LED on")
			led.Write(rpio.High)
		} else {
			log.Printf("Port 25 is LOW - LED off")
			led.Write(rpio.Low)
		}
		old = act
		time.Sleep(100 * time.Millisecond)
	}

}
