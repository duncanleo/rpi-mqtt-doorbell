package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	rpio "github.com/stianeikeland/go-rpio/v4"
)

func connect(clientID string, uri *url.URL) (mqtt.Client, error) {
	var opts = mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s", uri.Host))
	opts.SetUsername(uri.User.Username())
	password, _ := uri.User.Password()
	opts.SetPassword(password)
	opts.SetClientID(clientID)
	opts.CleanSession = false

	var client = mqtt.NewClient(opts)
	var token = client.Connect()
	for !token.WaitTimeout(3 * time.Second) {
	}
	return client, token.Error()
}

func main() {
	log.SetFlags(log.Lshortfile)

	var buttonGpioPin = flag.Int("buttonGpioPin", 17, "gpio pin for the button")
	var ledGpioPin = flag.Int("ledGpioPin", -1, "gpio pin for the LED")
	var brokerURI = flag.String("brokerURI", "mqtt://127.0.0.1:1883", "URI of the MQTT broker")
	var clientID = flag.String("clientID", "rpi-mqtt-doorbell", "client ID for MQTT")
	var topic = flag.String("topic", "rpi-mqtt-doorbell", "MQTT topic to publish")

	var lastPublish = time.Now()

	flag.Parse()

	err := rpio.Open()
	if err != nil {
		log.Fatal(err)
	}

	mqttURI, err := url.Parse(*brokerURI)
	if err != nil {
		log.Fatal(err)
	}

	client, err := connect(*clientID, mqttURI)
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

	var eventChan = make(chan bool)

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		log.Println("cleanup")
		rpio.Close()
		os.Exit(1)
	}()

	go func() {
		for isPressed := range eventChan {
			if time.Now().Sub(lastPublish).Seconds() <= 10 {
				continue
			}

			log.Printf("Button event! isPressed=%+v\n", isPressed)

			var message = "OFF"

			if isPressed {
				message = "ON"
			}

			token := client.Publish(*topic, 0, true, message)
			lastPublish = time.Now()
			if token.Error() != nil {
				log.Println(token.Error())
			}
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

	log.Println("Started... waiting for button press")

	// Button press loop
	var previousRpioState rpio.State
	for {
		var rpioState = rpioPin.Read()
		if previousRpioState == rpioState {
			previousRpioState = rpioState
			continue
		}
		if previousRpioState != rpioState {
			// Button press
			eventChan <- rpioState == rpio.Low
		}
		previousRpioState = rpioState
		time.Sleep(time.Millisecond * 100)
	}
}
