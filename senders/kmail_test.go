package kontur

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"testing"
	"time"

	"github.com/cbroglie/mustache"
	"github.com/moira-alert/moira"
	. "github.com/smartystreets/goconvey/convey"
)

func TestCreateRequest(t *testing.T) {
	location, _ := time.LoadLocation("UTC")
	sender := MailSender{
		URL:            "https://mail.testkontur.ru/v1/channels/moira-alerts/messages",
		Login:          "login",
		Password:       "password",
		FrontURI:       "http://localhost",
		Channel:        "moira-alerts",
		Template:       "devops-moira-fancy2",
		location:       location,
		DateTimeFormat: "15:04 02.01.2006",
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
			{"plot0.png", "plot0.png", "image/png", "AQAB"},
		}

		var body mailArgs
		err := json.NewDecoder(request.Body).Decode(&body)

		So(err, ShouldBeNil)
		So(body.Contents, ShouldResemble, expectedContents)
		So(body.Vars.PlotCIDs, ShouldResemble, []string{"plot0.png"})
	})

	Convey("Multiple plots", t, func() {
		request, _ := sender.createRequest(
			generateTestEvents(10, trigger.ID),
			contact,
			trigger,
			[][]byte{{1, 0, 1}, {1, 1, 0}, {1, 0, 0}},
			true,
		)
		expectedContents := []content{
			{"plot0.png", "plot0.png", "image/png", "AQAB"},
			{"plot1.png", "plot1.png", "image/png", "AQEA"},
			{"plot2.png", "plot2.png", "image/png", "AQAA"},
		}

		var body mailArgs
		err := json.NewDecoder(request.Body).Decode(&body)

		So(err, ShouldBeNil)
		So(body.Contents, ShouldResemble, expectedContents)
		So(body.Vars.PlotCIDs, ShouldResemble, []string{"plot0.png", "plot1.png", "plot2.png"})
	})
}

func TestRenderTemplateRequest(t *testing.T) {
	Convey("Render moira-fancy2.mustache", t, func() {
		filename := path.Join(path.Join(os.Getenv("PWD"), "kmail_templates"), "moira-fancy2.mustache")

		Convey("With one plot", func() {
			result, err := mustache.RenderFile(filename, map[string][]string{"plot_cids": {"plot0.png"}})
			So(err, ShouldBeNil)
			expectedPlotsPart := `<tr>
                                    <td class="align-center"
                                        style="box-sizing: border-box; font-family: 'Segoe UI', 'Helvetica Neue', Helvetica, Arial, sans-serif; font-size: 16px; vertical-align: top; font-weight: 500; text-align: center;"
                                        valign="top" align="center">
                                        <img src="cid:plot0.png" alt="Trigger plot"
                                             style="-ms-interpolation-mode: bicubic; max-width: 100%;">
                                    </td>
                                </tr>`
			So(result, ShouldContainSubstring, expectedPlotsPart)
		})

		Convey("With multiple plots", func() {
			result, err := mustache.RenderFile(filename, map[string][]string{"plot_cids": {"plot0.png", "plot1.png", "plot2.png"}})
			So(err, ShouldBeNil)
			expectedPlotsPart := `<tr>
                                    <td class="align-center"
                                        style="box-sizing: border-box; font-family: 'Segoe UI', 'Helvetica Neue', Helvetica, Arial, sans-serif; font-size: 16px; vertical-align: top; font-weight: 500; text-align: center;"
                                        valign="top" align="center">
                                        <img src="cid:plot0.png" alt="Trigger plot"
                                             style="-ms-interpolation-mode: bicubic; max-width: 100%;">
                                    </td>
                                </tr>
                                <tr>
                                    <td class="align-center"
                                        style="box-sizing: border-box; font-family: 'Segoe UI', 'Helvetica Neue', Helvetica, Arial, sans-serif; font-size: 16px; vertical-align: top; font-weight: 500; text-align: center;"
                                        valign="top" align="center">
                                        <img src="cid:plot1.png" alt="Trigger plot"
                                             style="-ms-interpolation-mode: bicubic; max-width: 100%;">
                                    </td>
                                </tr>
                                <tr>
                                    <td class="align-center"
                                        style="box-sizing: border-box; font-family: 'Segoe UI', 'Helvetica Neue', Helvetica, Arial, sans-serif; font-size: 16px; vertical-align: top; font-weight: 500; text-align: center;"
                                        valign="top" align="center">
                                        <img src="cid:plot2.png" alt="Trigger plot"
                                             style="-ms-interpolation-mode: bicubic; max-width: 100%;">
                                    </td>
                                </tr>`
			So(result, ShouldContainSubstring, expectedPlotsPart)
		})
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
