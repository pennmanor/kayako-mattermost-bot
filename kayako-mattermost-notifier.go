package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/pennmanor/gokayako"
	"github.com/pennmanor/gomm"
)

type Config struct {
	APIURL         string `json:"APIURL"`
	APIKey         string `json:"APIKey"`
	SecretKey      string `json:"SecretKey"`
	StaffURL       string `json:"StaffURL"`
	MattermostHook string `json:"MattermostHook"`
}

var config Config
var client gokayako.Kayako

func buildTicketString(k *gokayako.Kayako, t *gokayako.Ticket) string {

	tp, err := k.GetTicketPriorityByID(t.PriorityID)
	p := ""
	s := ""

	if err == nil {
		p = tp.Title
	} else {
		p = "Unknown Priority"
	}

	st, err := k.GetStaffByID(t.OwnerStaffID)

	if err == nil {
		s = st.FullName
	} else {
		s = "Nobody"
	}

	return fmt.Sprintf("[%v](%v/%v) [%v] %v by %v assigned to %v", t.DisplayID, config.StaffURL, t.ID, p, t.Subject, t.Email, s)

}

func watchTickets(c chan gomm.IncomingWebhookRequest) {

	oldTickets := make(map[string]gokayako.Ticket)

	deparments, err := client.GetDepartments()
	if err != nil {
		return
	}

	statusID, err := client.GetTicketStatusID("Open")
	if err != nil {
		return
	}

	for _, department := range deparments.Deparment {
		tickets, err := client.GetTickets(department.ID, statusID, -1, -1)
		if err != nil {
			return
		}

		for _, t := range tickets.Tickets {
			oldTickets[t.DisplayID] = t
		}

	}

	for {
		newTickets := make(map[string]gokayako.Ticket)

		for _, department := range deparments.Deparment {
			tickets, err := client.GetTickets(department.ID, statusID, -1, -1)
			if err != nil {
				return
			}

			for _, t := range tickets.Tickets {
				newTickets[t.DisplayID] = t
			}
		}

		for key, t := range oldTickets {
			if _, found := newTickets[key]; !found {
				msg := gomm.IncomingWebhookRequest{Text: fmt.Sprintf("Closed %v", buildTicketString(&client, &t)), Username: "kayako"}
				c <- msg
			}
		}

		for key, t := range newTickets {
			if _, found := oldTickets[key]; !found {
				msg := gomm.IncomingWebhookRequest{Text: fmt.Sprintf("Opened %v", buildTicketString(&client, &t)), Username: "kayako"}
				c <- msg
			}
		}

		oldTickets = newTickets
		time.Sleep(30 * time.Second)
	}

}

func main() {

	file, err := ioutil.ReadFile("config.json")
	if err != nil {
		os.Exit(1)
	}

	json.Unmarshal(file, &config)

	client.ApiKey = config.APIKey
	client.ApiUrl = config.APIURL
	client.SecretKey = config.SecretKey
	client.Client = &http.Client{}

	c := make(chan gomm.IncomingWebhookRequest)
	go watchTickets(c)

	mm := gomm.Mattermost{IncomingURL: config.MattermostHook}

	for {
		msg := <-c
		mm.Send(msg)
	}
}
