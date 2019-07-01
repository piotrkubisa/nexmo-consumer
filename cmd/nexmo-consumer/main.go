package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"html/template"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ses"
	prefixed "github.com/piotrkubisa/logrus-prefixed-formatter"
	log "github.com/sirupsen/logrus"
)

var (
	recipientEmail = os.Getenv("RECIPIENT_EMAIL")
)

func main() {
	// Initialize logrus
	InitializeLogging("DEBUG", "json")

	// Start lambda handler
	lambda.Start(Handler)
}

var Template = `<pre>{{ .Payload }}</pre>`

type TemplateData struct {
	Payload  string
	Receiver string
}

func Handler(ctx context.Context, e events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	lg := log.WithField("prefix", "Handler")

	if lctx, ok := lambdacontext.FromContext(ctx); ok {
		lg.WithField("reqID", lctx.AwsRequestID)
	}

	body := e.Body
	if e.IsBase64Encoded {
		b, err := base64.StdEncoding.DecodeString(body)
		if err != nil {
			return events.APIGatewayProxyResponse{}, err
		}
		body = string(b)
	}

	dict := map[string]interface{}{}
	if err := json.Unmarshal([]byte(body), &dict); err != nil {
		lg.WithField("body", dict).Infof("SMS received")
	} else {
		lg.WithField("body", body).Infof("SMS received")
	}

	sess, err := session.NewSession()
	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}
	cfg := aws.NewConfig().WithRegion("eu-west-1")
	svc := ses.New(sess, cfg)

	payload, err := json.MarshalIndent(dict, "", "  ")
	if err != nil {
		payload = []byte(body)
	}

	var tpl bytes.Buffer
	t := template.New("email")
	t = template.Must(t.Parse(Template))
	td := TemplateData{
		Payload:  string(payload),
		Receiver: recipientEmail,
	}
	err = t.Execute(&tpl, td)
	if err != nil {
		log.WithError(err).Error("Cannot render template")
	}

	out, err := svc.SendEmail(&ses.SendEmailInput{
		Message: &ses.Message{
			// Data: []byte(tpl.String()),
			Subject: &ses.Content{
				Charset: aws.String("UTF-8"),
				Data:    aws.String("Nexmo SMS"),
			},
			Body: &ses.Body{
				Html: &ses.Content{
					Data:    aws.String(tpl.String()),
					Charset: aws.String("UTF-8"),
				},
				Text: &ses.Content{
					Data:    aws.String(string(payload)),
					Charset: aws.String("UTF-8"),
				},
			},
		},
		Source: aws.String(td.Receiver),
		Destination: &ses.Destination{
			ToAddresses: []*string{aws.String(td.Receiver)},
		},
	})
	log.WithError(err).Error(out)

	resp := events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       "",
	}
	return resp, nil
}

// InitializeLogging sets logrus log level and formatting style.
func InitializeLogging(logLevel, logFormat string) {
	switch strings.ToLower(logFormat) {
	case "text":
		log.SetFormatter(new(prefixed.TextFormatter))
	default:
		log.SetFormatter(new(log.JSONFormatter))
	}

	// If log level cannot be resolved, exit gracefully
	if logLevel == "" {
		log.Warning("Log level could not be resolved, fallback to fatal")
		log.SetLevel(log.FatalLevel)
		return
	}
	// Parse level from string
	lvl, err := log.ParseLevel(logLevel)

	if err != nil {
		log.WithFields(log.Fields{
			"passed":  logLevel,
			"default": "fatal",
		}).Warn("Log level is not valid, fallback to default level")
		log.SetLevel(log.FatalLevel)
		return
	}

	log.SetLevel(lvl)
	log.WithFields(log.Fields{
		"level": logLevel,
	}).Debug("Log level successfully set")
}
