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
	TriggerState        moira.State         `json:"trigger_state"`
	TestNotification    bool                `json:"is_test"`
	PlotCID             string              `json:"plot_cid"`
	PlotCIDProvided     bool                `json:"plot_cid_provided"`
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
	trigger moira.TriggerData, plots [][]byte, throttled bool) error {
	req, err := sender.createRequest(events, contact, trigger, plots, throttled)
	if err != nil {
		return err
	}

	client := &http.Client{}
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

func (sender *MailSender) createRequest(events moira.NotificationEvents, contact moira.ContactData, trigger moira.TriggerData, plots [][]byte, throttled bool) (*http.Request, error) {
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
			"oldstate":    string(event.OldState),
			"state":       string(event.State),
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

	plotContents, plotCID := getPlotContents(plots)
	mailVars.PlotCID = plotCID
	mailVars.PlotCIDProvided = len(mailVars.PlotCID) > 0

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
		return nil, fmt.Errorf("failed to marshal json request body: %s", err)
	}

	req, err := http.NewRequest("POST", sender.URL, bytes.NewBuffer(argsJSON))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(sender.Login, sender.Password)
	req.Header.Add("Content-Type", "application/json")

	return req, nil
}

func getPlotContents(plots [][]byte) ([]content, string) {
	var plotCID string
	plotContents := make([]content, 0)
	if len(plots) > 0 {
		plot := plots[0]
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
