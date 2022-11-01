package nattraversal

import (
	"fmt"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
	"github.com/kungze/quic-tun/pkg/options"
	"github.com/pion/sdp/v3"
)

type MQTTClient struct {
	Client      mqtt.Client
	Topic       string
	RemoteTopic string
}

func NewMQTTClient(nt options.NATTraversalOptions, ch chan<- sdp.SessionDescription) MQTTClient {

	var messagePubHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
		fmt.Printf("Received message: %s from topic: %s\n", msg.Payload(), msg.Topic())
		remoteSD := new(sdp.SessionDescription)
		err := remoteSD.Unmarshal(msg.Payload())
		if err != nil {
			panic(err)
		}
		ch <- *remoteSD
	}

	var connectHandler mqtt.OnConnectHandler = func(client mqtt.Client) {
		fmt.Println("Connected")
	}

	var connectLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
		fmt.Printf("Connect lost: %v", err)
	}

	opts := mqtt.NewClientOptions()
	opts.SetClientID(uuid.NewString())
	opts.AddBroker(fmt.Sprintf("tcp://%s", nt.MQTTServerURL))
	opts.SetUsername(nt.MQTTServerUsername)
	opts.SetPassword(nt.MQTTServerPassword)
	opts.SetDefaultPublishHandler(messagePubHandler)
	opts.OnConnect = connectHandler
	opts.OnConnectionLost = connectLostHandler
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
	return MQTTClient{
		Client: client,
		Topic:  fmt.Sprintf("kungze.com/quic-tun/%s", nt.MQTTTopicKey),
	}
}

func Subscribe(client MQTTClient) {
	token := client.Client.Subscribe(client.Topic, 1, nil)
	token.Wait()
	fmt.Printf("Subscribed to topic: %s\n", client.Topic)
}

func Publish(client MQTTClient, payload any) {
	token := client.Client.Publish(client.RemoteTopic, 0, false, payload)
	token.Wait()
}
