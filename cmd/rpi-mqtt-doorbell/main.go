package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	rpio "github.com/stianeikeland/go-rpio/v4"
)

// Adapted from https://play.golang.org/p/N4TTxDuWrFo
func debounce(interval time.Duration, eventChan chan string, callback func()) {
	var event string
	for range eventChan {
	L:
		for {
			select {
			case event = <-eventChan:
				// Do nothing
			case <-time.After(interval):
				if event != "" {
					callback()
					break L
				}
			}
		}
	}
}

func main() {
	log.SetFlags(log.Lshortfile)

	var buttonGpioPin = flag.Int("buttonGpioPin", 17, "gpio pin for the button")
	var ledGpioPin = flag.Int("ledGpioPin", -1, "gpio pin for the LED")

	flag.Parse()

	err := rpio.Open()
	if err != nil {
		log.Fatal(err)
	}

	rpioPin := rpio.Pin(*buttonGpioPin)
	rpioPin.Input()
	rpioPin.PullUp()

	if *ledGpioPin != -1 {
		ledRpioPin := rpio.Pin(*ledGpioPin)
		ledRpioPin.Output()
		ledRpioPin.PullDown()
	}

	var eventChan = make(chan string)

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		log.Println("cleanup")
		rpio.Close()
		os.Exit(1)
	}()

	go debounce(1*time.Second, eventChan, func() {
		log.Println("Button press!")
	})

	// Button press loop
	go func() {
		var previousRpioState rpio.State
		for {
			var rpioState = rpioPin.Read()
			if rpioState == rpio.Low && previousRpioState != rpioState {
				// Button press
				eventChan <- "press"
			}
			previousRpioState = rpioState
			time.Sleep(time.Millisecond * 100)
		}
	}()

	// LED loop
	go func() {
		if *ledGpioPin == -1 {
			return
		}
		ledRpioPin := rpio.Pin(*ledGpioPin)
		for {
			if rpioPin.Read() == rpio.Low {
				ledRpioPin.High()
			} else {
				ledRpioPin.Low()
			}
			time.Sleep(time.Millisecond * 50)
		}
	}()
}
