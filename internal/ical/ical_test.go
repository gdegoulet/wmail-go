package ical

import (
	"strings"
	"testing"
	"time"
)

const invite = "BEGIN:VCALENDAR\r\nMETHOD:REQUEST\r\nBEGIN:VTIMEZONE\r\nTZID:Europe/Paris\r\nEND:VTIMEZONE\r\nBEGIN:VEVENT\r\n" +
	"SUMMARY:Réunion de\r\n  projet\\, phase 2\r\nDTSTART;TZID=Europe/Paris:20261105T140000\r\nDTEND:20261105T133000Z\r\n" +
	"LOCATION:Salle 3\\nBâtiment A\r\nORGANIZER;CN=\"Durand, Anne\":mailto:anne@x.org\r\n" +
	"ATTENDEE;CN=Paul;PARTSTAT=ACCEPTED:mailto:paul@x.org\r\nATTENDEE:mailto:zoe@x.org\r\nRRULE:FREQ=WEEKLY\r\n" +
	"BEGIN:VALARM\r\nSUMMARY:rappel ignoré\r\nEND:VALARM\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"

func TestParseInvitation(t *testing.T) {
	paris, _ := time.LoadLocation("Europe/Paris")
	ev := Parse([]byte(invite), time.UTC)
	if len(ev) != 1 {
		t.Fatalf("%d événements", len(ev))
	}
	e := ev[0]
	if e.Summary != "Réunion de projet, phase 2" || e.Location != "Salle 3\nBâtiment A" || e.Method != "REQUEST" || e.Recurrence != "FREQ=WEEKLY" {
		t.Errorf("champs : %+v", e)
	}
	if !e.Start.Equal(time.Date(2026, 11, 5, 14, 0, 0, 0, paris)) {
		t.Errorf("début avec TZID : %v", e.Start)
	}
	if !e.End.Equal(time.Date(2026, 11, 5, 13, 30, 0, 0, time.UTC)) {
		t.Errorf("fin UTC : %v", e.End)
	}
	if e.Organizer.Name != "Durand, Anne" || e.Organizer.Email != "anne@x.org" {
		t.Errorf("organisateur : %+v", e.Organizer)
	}
	if len(e.Attendees) != 2 || e.Attendees[0].Status != "ACCEPTED" || e.Attendees[1].Label() != "zoe@x.org" {
		t.Errorf("participants : %+v", e.Attendees)
	}
}

func TestAllDayDurationAndLinks(t *testing.T) {
	ev := Parse([]byte("BEGIN:VEVENT\nSUMMARY:Congé\nDTSTART;VALUE=DATE:20261224\nEND:VEVENT\nBEGIN:VEVENT\nSUMMARY:Appel\nDTSTART:20261105T090000Z\nDURATION:PT1H30M\nEND:VEVENT\n"), time.UTC)
	if len(ev) != 2 || !ev[0].AllDay || ev[1].End.Sub(ev[1].Start) != 90*time.Minute {
		t.Fatalf("%+v", ev)
	}
	g := ev[0].GoogleURL()
	if !strings.Contains(g, "dates=20261224%2F20261225") || !strings.HasPrefix(g, "https://www.google.com/calendar/render?") {
		t.Errorf("Google (jour entier) : %s", g)
	}
	o := ev[1].OutlookURL()
	if !strings.Contains(o, "startdt=2026-11-05T09%3A00%3A00Z") || !strings.Contains(o, "enddt=2026-11-05T10%3A30%3A00Z") {
		t.Errorf("Outlook : %s", o)
	}
	if (Event{}).GoogleURL() != "" {
		t.Error("lien sans date")
	}
}

func TestRobustness(t *testing.T) {
	for _, in := range []string{"", "n'importe quoi", "BEGIN:VEVENT\nDTSTART:pas une date\nEND:VEVENT", "BEGIN:VEVENT", strings.Repeat("BEGIN:VEVENT\nEND:VEVENT\n", 20)} {
		if n := len(Parse([]byte(in), nil)); n > MaxEvents {
			t.Errorf("%d événements", n)
		}
	}
	if got := len(Parse([]byte(strings.Repeat("BEGIN:VEVENT\nSUMMARY:x\nEND:VEVENT\n", 20)), nil)); got != MaxEvents {
		t.Errorf("limite : %d", got)
	}
}
