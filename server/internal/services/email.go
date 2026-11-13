package services

import (
	"bytes"
	"context"
	"expenser/internal/models"
	"expenser/internal/utilities"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/google/uuid"
)

// EmailMessage is one email, already rendered.
type EmailMessage struct {
	To      string
	ToName  string // optional name shown with the address in the To header
	Subject string
	Text    string // plain-text body
	HTML    string // HTML body
}

// Mailer delivers rendered emails (SMTPMailer in production).
type Mailer interface {
	Send(ctx context.Context, m EmailMessage) error
}

// EmailNotifier sends reminders as emails in the bill owner's language.
type EmailNotifier struct {
	mailer Mailer
	links  *ReminderLinkBuilder // nil: no links in emails
}

func NewEmailNotifier(m Mailer, links *ReminderLinkBuilder) *EmailNotifier {
	return &EmailNotifier{mailer: m, links: links}
}

func (n *EmailNotifier) Notify(ctx context.Context, r Reminder) error {
	if r.To == "" {
		return fmt.Errorf("reminder for %s has no recipient", r.Payment.ID)
	}
	if n.links != nil && r.Links == nil {
		r.Links = n.links.For(r.Payment)
	}
	return n.mailer.Send(ctx, BuildReminderEmail(r))
}

// emailField is one "label: value" line of a reminder email.
type emailField struct {
	Label string
	Value string
}

// BuildReminderEmail renders the subject and bodies for a reminder. There
// is deliberately no link to the app: the server is not reachable from
// the internet.
func BuildReminderEmail(r Reminder) EmailMessage {
	p := r.Payment
	lang := p.UserLang
	if lang == "" {
		lang = "en"
	}
	t := func(key string) string { return utilities.T(lang, key) }

	label := strings.TrimSpace(p.Name)
	if label == "" {
		label = t("type." + p.TypeName)
	}
	when := reminderDueText(lang, r.DaysLeft, p.DateKind == models.DateArrives)

	dateLabel := t("bills.date.pay_by")
	if p.DateKind == models.DateArrives {
		dateLabel = t("bills.date.arrives")
	}
	fields := []emailField{
		{t("bills.type"), t("type." + p.TypeName)},
	}
	if p.ExpectedAmount != nil {
		fields = append(fields, emailField{t("common.amount"), utilities.Money(*p.ExpectedAmount)})
	}
	fields = append(fields, emailField{dateLabel, p.DueDate.Format("02.01.2006") + " (" + when + ")"})
	if p.PaymentHours != "" {
		fields = append(fields, emailField{t("bills.payment_hours"), p.PaymentHours})
	}
	if len(p.PaymentOptions) > 0 {
		opts := make([]string, 0, len(p.PaymentOptions))
		for _, o := range p.PaymentOptions {
			if tr := t("bills.option." + o); tr != "bills.option."+o {
				o = tr
			}
			opts = append(opts, o)
		}
		fields = append(fields, emailField{t("bills.payment_options"), strings.Join(opts, ", ")})
	}
	if p.Notes != "" {
		fields = append(fields, emailField{t("common.notes"), p.Notes})
	}

	greeting := fmt.Sprintf(t("email.greeting"), p.GreetingName())
	var text strings.Builder
	fmt.Fprintf(&text, "%s\n\n%s\n\n%s\n", greeting, t("email.intro"), label)
	for _, f := range fields {
		fmt.Fprintf(&text, "%s: %s\n", f.Label, f.Value)
	}
	if l := r.Links; l != nil {
		fmt.Fprintf(&text, "\n%s: %s\n", t("email.open_bills"), l.BillsURL)
		if len(l.Snooze) > 0 {
			fmt.Fprintf(&text, "\n%s\n", t("email.snooze"))
			for _, s := range l.Snooze {
				fmt.Fprintf(&text, "- %s: %s\n", snoozeLabel(lang, s.Days), s.URL)
			}
		}
		fmt.Fprintf(&text, "\n%s\n", t("email.links_note"))
	}
	fmt.Fprintf(&text, "\n%s\n", t("email.footer"))

	var html bytes.Buffer
	_ = reminderEmailHTML.Execute(&html, map[string]any{
		"Greeting": greeting, "Intro": t("email.intro"), "Label": label, "When": when,
		"Fields": fields, "Footer": t("email.footer"), "Lang": lang,
		"Links": r.Links, "OpenBills": t("email.open_bills"), "SnoozeTitle": t("email.snooze"),
		"SnoozeLabels": snoozeLabels(lang, r.Links), "LinksNote": t("email.links_note"),
	})

	return EmailMessage{
		To:      r.To,
		ToName:  p.RecipientName(r.To),
		Subject: fmt.Sprintf(t("email.subject"), label, when),
		Text:    text.String(),
		HTML:    html.String(),
	}
}

// reminderDueText is the "in 3 days" / "waiting 2 days" / "3 days overdue"
// phrase, matching the Bills page.
func reminderDueText(lang string, days int, arrives bool) string {
	t := func(key string) string { return utilities.T(lang, key) }
	if arrives {
		switch {
		case days == 0:
			return t("bills.due.arrives_today")
		case days == 1:
			return t("bills.due.arrives_tomorrow")
		case days > 1:
			return fmt.Sprintf(t("bills.due.arrives_in"), days)
		case days == -1:
			return t("bills.due.waiting_one")
		default:
			return fmt.Sprintf(t("bills.due.waiting"), -days)
		}
	}
	switch {
	case days == 0:
		return t("bills.due.today")
	case days == 1:
		return t("bills.due.tomorrow")
	case days > 1:
		return fmt.Sprintf(t("bills.due.in"), days)
	case days == -1:
		return t("bills.due.overdue_one")
	default:
		return fmt.Sprintf(t("bills.due.overdue"), -days)
	}
}

// reminderEmailHTML is a small, inline-styled email (mail clients ignore
// most stylesheets).
var reminderEmailHTML = template.Must(template.New("reminder").Parse(`<!doctype html>
<html lang="{{ .Lang }}"><body style="margin:0;padding:24px;background:#f4f5f9;font-family:Arial,Helvetica,sans-serif;color:#1d2433">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:520px;margin:0 auto;background:#ffffff;border-radius:12px;border:1px solid #e3e6ef">
<tr><td style="padding:24px 24px 8px">
<p style="margin:0 0 12px;font-size:15px">{{ .Greeting }}</p>
<p style="margin:0 0 16px;font-size:15px;color:#5b6478">{{ .Intro }}</p>
<p style="margin:0;font-size:20px;font-weight:bold">{{ .Label }}</p>
<p style="margin:6px 0 16px"><span style="display:inline-block;padding:3px 10px;border-radius:999px;background:#e8edff;color:#2f5bea;font-size:13px;font-weight:bold">{{ .When }}</span></p>
</td></tr>
<tr><td style="padding:0 24px 16px">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="font-size:14px">
{{ range .Fields }}<tr><td style="padding:8px 0;border-top:1px solid #eef0f5;color:#5b6478;width:40%;vertical-align:top">{{ .Label }}</td><td style="padding:8px 0;border-top:1px solid #eef0f5;font-weight:bold;vertical-align:top;white-space:pre-line">{{ .Value }}</td></tr>
{{ end }}</table>
</td></tr>
{{ with .Links }}<tr><td align="center" style="padding:4px 24px 8px;text-align:center">
<a href="{{ .BillsURL }}" style="display:inline-block;padding:10px 18px;border-radius:999px;background:#2f5bea;color:#ffffff;font-size:14px;font-weight:bold;text-decoration:none">{{ $.OpenBills }}</a>
</td></tr>
{{ if .Snooze }}<tr><td align="center" style="padding:12px 24px 0;font-size:13px;color:#5b6478;text-align:center">{{ $.SnoozeTitle }}
<div style="margin-top:8px">{{ range $i, $s := .Snooze }}<a href="{{ $s.URL }}" style="display:inline-block;margin:0 3px 6px;padding:6px 12px;border-radius:999px;border:1px solid #c9d2ea;color:#2f5bea;font-size:13px;font-weight:bold;text-decoration:none">{{ index $.SnoozeLabels $i }}</a>{{ end }}</div>
</td></tr>{{ end }}
<tr><td align="center" style="padding:8px 24px 0;font-size:12px;color:#8a92a6;text-align:center">{{ $.LinksNote }}</td></tr>{{ end }}
<tr><td style="padding:16px 24px 24px;font-size:12px;color:#8a92a6">{{ .Footer }}</td></tr>
</table></body></html>`))

// BuildTestReminderEmail is a sample reminder with a "[Test]" subject, so
// the SMTP setup and the email's look can be checked without waiting for a
// real reminder. It greets and addresses the user like their reminders
// would. Every field the email can show is filled, with long values,
// a custom payment option, a multi-line note and HTML-like characters, to
// see how all of it renders. Dev mode only.
func BuildTestReminderEmail(username, displayName, lang, to string, today time.Time, links *ReminderLinkBuilder) EmailMessage {
	if lang == "" {
		lang = "en"
	}
	amount := 1234.56
	sampleRepeat := 3
	r := Reminder{
		To:       to,
		DaysLeft: 3,
		Payment: models.RecurringPayment{
			Username: username, UserDisplayName: displayName, UserEmail: to, Name: "Vivacom Fiber 500 Mbps + TV (apartment & garage)",
			TypeName: "Internet", Category: models.SectionHouse,
			DateKind: models.DatePayBy, DueDate: today.AddDate(0, 0, 3),
			ExpectedAmount: &amount,
			PaymentHours:   "Mon–Fri 9:00–18:00, Sat 10:00–14:00",
			PaymentOptions: []string{"cash", "card", "bank_transfer", "online", "ePay.bg"},
			Notes: "Client no. 1234567 — pay at the office on <Vitosha 15> or via ePay.\n" +
				"Second line: ask for the \"loyalty\" discount & keep the receipt.",
			UserLang: lang, ID: uuid.New(), RepeatEveryDays: &sampleRepeat,
		},
	}
	if links != nil {
		// Snooze links in the sample point at a bill that doesn't exist,
		// so they open the "out of date" page.
		r.Links = links.For(r.Payment)
	}
	m := BuildReminderEmail(r)
	m.Subject = utilities.T(lang, "email.test_prefix") + " " + m.Subject
	return m
}

// snoozeLabel is a snooze button's text: "1 day", "3 days", "1 week".
func snoozeLabel(lang string, days int) string {
	switch {
	case days == 7:
		return utilities.T(lang, "email.snooze_week")
	case days == 1:
		return utilities.T(lang, "email.snooze_day")
	}
	return fmt.Sprintf(utilities.T(lang, "email.snooze_days"), days)
}

func snoozeLabels(lang string, l *ReminderLinks) []string {
	if l == nil {
		return nil
	}
	out := make([]string, len(l.Snooze))
	for i, s := range l.Snooze {
		out[i] = snoozeLabel(lang, s.Days)
	}
	return out
}
