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

type shortenerArgs struct {
	longUrl  string `json:"long_url"`
	shortUrl string `json:"link"`
}

// SmsSender implements moira sender interface via kontur mail
type SmsSender struct {
	URL           string
	Login         string
	Password      string
	FrontURI      string
	shortenerURL  string
	shortenerAuth string
	client        *http.Client
	location      *time.Location
	logger        moira.Logger
}

// Init read yaml config
func (sender *SmsSender) Init(senderSettings map[string]string, logger moira.Logger, location *time.Location, dateTimeFormat string) error {
	sender.URL = senderSettings["url"]
	sender.Login = senderSettings["login"]
	sender.Password = senderSettings["password"]
	sender.FrontURI = senderSettings["front_uri"]
	sender.shortenerURL = senderSettings["shortener_url"]
	sender.shortenerAuth = senderSettings["shortener_auth"]
	sender.location = location
	sender.logger = logger
	sender.client = &http.Client{
		Timeout:   30 * time.Second,
		Transport: &http.Transport{DisableKeepAlives: true},
	}
	return nil
}

// SendEvents implements Sender interface Send
func (sender *SmsSender) SendEvents(events moira.NotificationEvents, contact moira.ContactData, trigger moira.TriggerData, plot []byte, throttled bool) error {
	link := trigger.GetTriggerURI(sender.FrontURI)
	if err := sender.simplifyLink(&link); err != nil {
		sender.logger.Warningf(err.Error())
	}
	phoneNumber, err := buildPhoneNumber(contact)
	if err != nil {
		return err
	}
	message := buildMessage(events, trigger, throttled, link)
	return sender.sendSms(phoneNumber, message)
}

func buildPhoneNumber(contact moira.ContactData) (string, error) {
	phoneNumber := "+7" + contact.Value
	if !regexp.MustCompile(`^\+79[0-9]{9}$`).MatchString(phoneNumber) {
		return "", fmt.Errorf("invalid phone number: %s", phoneNumber)
	}
	return phoneNumber, nil
}

func buildMessage(events moira.NotificationEvents, trigger moira.TriggerData, throttled bool, link string) string {
	const maxMessageSize = 280
	const truncatedText = "...and %d\n"
	const throtledText = "throttled\n"

	maxMessageEventsSize := maxMessageSize - len(link) - len(truncatedText)
	if throttled {
		maxMessageEventsSize -= len(throtledText)
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
	smsMessage.WriteString(link)
	return smsMessage.String()
}

func (sender *SmsSender) sendSms(phoneNumber, message string) error {
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
	reqStruct := &smsArgs{
		Text:             message,
		DestionationVars: *smsDestinationVars,
		SourceVars:       *smsSourceVars,
		DeliveryControl:  true,
	}
	reqJSON, err := json.Marshal(reqStruct)
	if err != nil {
		return fmt.Errorf("failed to marshal json request body: %s", err)
	}
	log.Debugf("calling kontur.sms with body %s", string(reqJSON))

	request, err := http.NewRequest("POST", sender.URL, bytes.NewBuffer(reqJSON))
	request.SetBasicAuth(sender.Login, sender.Password)
	request.Header.Add("Content-Type", "application/json")

	response, err := sender.client.Do(request)
	if response != nil {
		defer response.Body.Close()
	}
	if err != nil {
		return fmt.Errorf("failed to call kontur.sms: %s", err)
	}
	if response.StatusCode != 201 {
		log.Warning("kontur.sms replied with %s", response.Status)
	}
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("kontur.sms failed read body: %s", err)
	}
	log.Debugf("kontur.sms answer:\n%s", string(body))
	return nil
}

func (sender *SmsSender) simplifyLink(longLink *string) error {
	if sender.shortenerURL == "" && sender.shortenerAuth == "" {
		return nil
	}
	reqStruct := shortenerArgs{longUrl: *longLink}
	reqJSON, err := json.Marshal(reqStruct)
	if err != nil {
		return err
	}
	request, err := http.NewRequest("POST", sender.shortenerURL, bytes.NewBuffer(reqJSON))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", sender.shortenerAuth)

	response, err := sender.client.Do(request)
	if response != nil {
		defer response.Body.Close()
	}
	if err != nil {
		return err
	}
	if response.StatusCode != 201 {
		return fmt.Errorf("shortener api replied with %s", response.Status)
	}
	var respStruct shortenerArgs
	respJSON, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}
	err = json.Unmarshal(respJSON, &respStruct)
	if err != nil {
		return err
	}

	longLink = &respStruct.shortUrl
	return nil
}