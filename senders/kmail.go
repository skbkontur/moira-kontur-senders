package kontur

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/AlexAkulov/go-humanize"

	"github.com/moira-alert/moira"
)

// MailSender implements moira sender interface via kontur mail
type MailSender struct {
	URL            string
	Login          string
	Password       string
	FrontURI       string
	Channel        string
	Template       string
	location       *time.Location
	DateTimeFormat string
}

type mailArgs struct {
	Channel  string               `json:"channel"`
	Address  string               `json:"address"`
	Vars     mailNotificationVars `json:"vars"`
	Template string               `json:"template"`
	Subject  string               `json:"subject"`
	Contents []content            `json:"contents,omitempty"`
}

type mailNotificationVars struct {
	Link                string              `json:"link"`
	Throttled           bool                `json:"throttled"`
	Rows                []map[string]string `json:"rows"`
	Description         string              `json:"desc"`
	DescriptionProvided bool                `json:"desc_provided"`
	TriggerName         string              `json:"name"`
	Tags                string              `json:"tags"`
	TriggerState        string              `json:"trigger_state"`
	TestNotification    bool                `json:"is_test"`
	PlotCID             string              `json:"plot_cid"`
}

type content struct {
	ContentID   string `json:"id"`
	ContentName string `json:"name"`
	ContentType string `json:"type"`
	ContentData string `json:"data"`
}

var log moira.Logger

// Init reads yaml config
func (sender *MailSender) Init(senderSettings map[string]string, logger moira.Logger,
	location *time.Location, dateTimeFormat string) error {

	sender.URL = senderSettings["url"]
	sender.Login = senderSettings["login"]
	sender.Password = senderSettings["password"]
	sender.Channel = senderSettings["channel"]
	sender.Template = senderSettings["template"]
	log = logger
	sender.FrontURI = senderSettings["front_uri"]
	sender.location = location
	sender.DateTimeFormat = dateTimeFormat
	return nil
}

// SendEvents implements Sender interface Send
func (sender *MailSender) SendEvents(events moira.NotificationEvents, contact moira.ContactData,
	trigger moira.TriggerData, plot []byte, throttled bool) error {

	mailVars := &mailNotificationVars{}
	mailVars.Link = fmt.Sprintf("%s/trigger/%s", sender.FrontURI, events[0].TriggerID)
	mailVars.Throttled = throttled
	for _, event := range events {
		var value string
		if useFloat64(event.Value) >= 1000 {
			value = humanize.SI(useFloat64(event.Value), 10, "")
		} else {
			value = humanize.Ftoa(useFloat64(event.Value), 10)
		}
		vars := map[string]string{
			"metric":      event.Metric,
			"timestamp":   time.Unix(event.Timestamp, 0).In(sender.location).Format(sender.DateTimeFormat),
			"oldstate":    event.OldState,
			"state":       event.State,
			"value":       value,
			"warn_value":  strconv.FormatFloat(trigger.WarnValue, 'f', -1, 64),
			"error_value": strconv.FormatFloat(trigger.ErrorValue, 'f', -1, 64),
			"message":     useString(event.Message),
		}
		mailVars.Rows = append(mailVars.Rows, vars)
	}

	state := events.GetSubjectState()
	mailVars.TriggerState = state
	tags := trigger.GetTags()
	subject := fmt.Sprintf("%s %s %s", state, trigger.Name, tags)

	if state == "TEST" {
		mailVars.TestNotification = true
	}

	mailVars.TriggerName = trigger.Name
	mailVars.Tags = tags

	if trigger.Desc != "" {
		mailVars.Description = formatDescription(trigger.Desc)
		mailVars.DescriptionProvided = true
	}

	plotContents, plotCID := getPlotContents(plot)
	mailVars.PlotCID = plotCID

	args := &mailArgs{
		Channel:  sender.Channel,
		Address:  contact.Value,
		Template: sender.Template,
		Vars:     *mailVars,
		Subject:  subject,
		Contents: plotContents,
	}

	argsJSON, err := json.Marshal(args)
	if err != nil {
		return fmt.Errorf("failed to marshal json request body: %s", err)
	}

	log.Debugf("Calling kontur.spam with body %s", string(argsJSON))

	client := &http.Client{}
	req, err := http.NewRequest("POST", sender.URL, bytes.NewBuffer(argsJSON))
	req.SetBasicAuth(sender.Login, sender.Password)
	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call kontur.spam: %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 400 {
		log.Errorf("Delete message! kontur.spam replied with error: %s", resp.Status)
		return nil
	}
	if resp.StatusCode != 201 {
		return fmt.Errorf("kontur.spam replied with error: %s", resp.Status)
	}
	return nil
}

// GetLocation implements Sender interface GetLocation
func (sender *MailSender) GetLocation() *time.Location {
	return sender.location
}

func getPlotContents(plot []byte) ([]content, string) {
	var plotCID string
	plotContents := make([]content, 0)
	if len(plot) > 0 {
		plotCID = "plot.png"
		plotContent := content{
			ContentID:   plotCID,
			ContentName: plotCID,
			ContentType: "image/png",
			ContentData: fromBytesToBase64(plot),
		}
		plotContents = append(plotContents, plotContent)
	}
	return plotContents, plotCID
}

func formatDescription(desc string) string {
	escapedDesc := html.EscapeString(desc)
	escapedDesc = strings.Replace(escapedDesc, "\n", "\n<br/>", -1)

	return escapedDesc
}