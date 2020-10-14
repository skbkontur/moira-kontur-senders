package kontur

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/moira-alert/moira"
	. "github.com/smartystreets/goconvey/convey"
)

func TestCreateRequest(t *testing.T) {
	location, _ := time.LoadLocation("UTC")
	sender := MailSender{
		URL:            "url",
		Login:          "login",
		Password:       "password",
		FrontURI:       "http://localhost",
		Channel:        "channel",
		Template:       "template",
		location:       location,
		DateTimeFormat: "datetimeformat",
	}
	trigger := moira.TriggerData{
		ID:         "triggerID-0000000000001",
		Name:       "test trigger 1",
		Targets:    []string{"test.target.1"},
		WarnValue:  10,
		ErrorValue: 20,
		Tags:       []string{"test-tag-1"},
		Desc: `# header 1
some text **bold text**
## header 2
some other text _italics text_`,
	}
	contact := moira.ContactData{
		ID:    "ContactID-000000000000001",
		Type:  "email",
		Value: "mail1@example.com",
	}

	Convey("One plot", t, func() {
		request, _ := sender.createRequest(
			generateTestEvents(10, trigger.ID),
			contact,
			trigger,
			[][]byte{{1, 0, 1}},
			true,
		)
		expectedContents := []content{
			{"plot.png", "plot.png", "image/png", "AQAB"},
		}

		var body mailArgs
		err := json.NewDecoder(request.Body).Decode(&body)

		So(err, ShouldBeNil)
		So(body.Contents, ShouldResemble, expectedContents)
		So(body.Vars.PlotCID, ShouldResemble, "plot.png")
	})
}

func generateTestEvents(n int, subscriptionID string) []moira.NotificationEvent {
	events := make([]moira.NotificationEvent, 0, n)
	for i := 0; i < n; i++ {
		event := moira.NotificationEvent{
			Metric:         fmt.Sprintf("Metric number #%d", i),
			SubscriptionID: &subscriptionID,
			State:          moira.StateTEST,
		}
		events = append(events, event)
	}
	return events
}
