package kontur

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"time"

	"github.com/AlexAkulov/go-humanize"
	"github.com/hiraq-golang/googl-shortener"
	"github.com/moira-alert/moira"
)

type smsArgs struct {
	Text             string         `json:"text"`
	DestionationVars smsAddressArgs `json:"destinationAddress"`
	SourceVars       smsAddressArgs `json:"sourceAddress"`
	DeliveryControl  bool           `json:"deliveryControl"`
}

type smsAddressArgs struct {
	Address string  `json:"address"`
	Npi     float64 `json:"npi"`
	Ton     float64 `json:"ton"`
}

// SmsSender implements moira sender interface via kontur mail
type SmsSender struct {
	URL      string
	Login    string
	Password string
	FrontURI string
	GooglKey string
	location *time.Location
}

//Init read yaml config
func (sender *SmsSender) Init(senderSettings map[string]string, logger moira.Logger, location *time.Location, dateTimeFormat string) error {
	sender.URL = senderSettings["url"]
	sender.Login = senderSettings["login"]
	sender.Password = senderSettings["password"]
	log = logger
	sender.FrontURI = senderSettings["front_uri"]
	sender.GooglKey = senderSettings["googl_key"]
	sender.location = location
	return nil
}

//SendEvents implements Sender interface Send
func (sender *SmsSender) SendEvents(events moira.NotificationEvents, contact moira.ContactData, trigger moira.TriggerData, throttled bool) error {
	const maxMessageSize = 280
	const truncatedText = "...and %d\n"
	const throtledText = "throttled\n"
	var triggerLongLink, triggerLink string
	var err error
	if events[0].State == "TEST" {
		triggerLongLink = fmt.Sprintf("%s", sender.FrontURI)
	} else {
		triggerLongLink = fmt.Sprintf("%s/trigger/%s", sender.FrontURI, events[0].TriggerID)
		triggerLink, err = googl.ShortIt(sender.GooglKey, triggerLongLink)
		if err != nil {
			log.Warningf("Can't shorting url, %s", err.Error())
			triggerLink = triggerLongLink
		}
	}
	maxMessageEventsSize := maxMessageSize - len(triggerLink) - len(truncatedText)
	if throttled {
		maxMessageEventsSize -= len(throtledText)
	}
	phoneNumber := "+7" + contact.Value
	if !regexp.MustCompile(`^\+79[0-9]{9}$`).MatchString(phoneNumber) {
		return fmt.Errorf("invalid phone number: %s", phoneNumber)
	}

	var smsMessage bytes.Buffer
	smsMessage.WriteString(fmt.Sprintf("%.40s\n", trigger.Name))
	for i, event := range events {
		metricName := event.Metric
		if len(metricName) > 20 {
			metricName = fmt.Sprintf("%.18s..", event.Metric)
		}
		var value string
		if useFloat64(event.Value) >= 1000 {
			value = humanize.SI(useFloat64(event.Value), 2, "")
		} else {
			value = humanize.Ftoa(useFloat64(event.Value), 2)
		}
		eventLine := fmt.Sprintf("%s %s %s\n",
			event.State,
			metricName,
			value,
		)
		if smsMessage.Len()+len(eventLine) > maxMessageEventsSize && len(events) > i+1 {
			smsMessage.WriteString(fmt.Sprintf(truncatedText, len(events)-i))
			break
		}
		smsMessage.WriteString(eventLine)
	}
	if throttled {
		smsMessage.WriteString(throtledText)
	}
	smsMessage.WriteString(triggerLink)

	smsSourceVars := &smsAddressArgs{
		Address: "kontur",
		Npi:     1,
		Ton:     5,
	}
	smsDestinationVars := &smsAddressArgs{
		Address: phoneNumber,
		Npi:     1,
		Ton:     1,
	}

	args := &smsArgs{
		Text:             smsMessage.String(),
		DestionationVars: *smsDestinationVars,
		SourceVars:       *smsSourceVars,
		DeliveryControl:  true,
	}
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return fmt.Errorf("failed to marshal json request body: %s", err)
	}
	log.Debugf("calling kontur.sms with body %s", string(argsJSON))

	client := &http.Client{}

	req, err := http.NewRequest("POST", sender.URL, bytes.NewBuffer(argsJSON))
	req.SetBasicAuth(sender.Login, sender.Password)
	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call kontur.sms: %s", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		log.Warning("kontur.sms replied with error %s", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("kontur.sms failed read body: %s", err)
	}
	log.Debugf("kontur.sms answer:\n%s", string(body))
	return nil
}
