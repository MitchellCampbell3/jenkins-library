package ans

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/SAP/jenkins-library/pkg/body"
	"github.com/SAP/jenkins-library/pkg/xsuaa"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"net/http"
)

const (
	authHeaderKey = "Authorization"

	infoSeverity    = "INFO"
	noticeSeverity  = "NOTICE"
	warningSeverity = "WARNING"
	errorSeverity   = "ERROR"
	fatalSeverity   = "FATAL"

	exceptionCategory    = "EXCEPTION"
	alertCategory        = "ALERT"
	notificationCategory = "NOTIFICATION"
)

// ANS holds the setup for the xsuaa service to retrieve a bearer token for authorization and
// the URL to the SAP Alert Notification Service backend
type ANS struct {
	XSUAA xsuaa.XSUAA
	URL   string
}

// Configuration defines the configuration options for the SAP Alert Notification Service on BTP
type Configuration struct {
	ServiceKey            string `json:"serviceKey,omitempty"`
	EventTemplateFilePath string `json:"eventTemplateFilePath,omitempty"`
	EventTemplate         string `json:"eventTemplate,omitempty"`
}

// Client to send the event to the SAP Alert Notification Service
type Client interface {
	Send(event Event) error
}

// ServiceKey holds the information about the SAP Alert Notification Service to send the events to
type ServiceKey struct {
	Url          string `json:"url"`
	ClientId     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	OauthUrl     string `json:"oauth_url"`
}

// Event structure of the SAP Alert Notification Service
type Event struct {
	EventType      string                 `json:"eventType,omitempty"`
	EventTimestamp int64                  `json:"eventTimestamp,omitempty"`
	Severity       string                 `json:"severity,omitempty"`
	Category       string                 `json:"category,omitempty"`
	Subject        string                 `json:"subject,omitempty"`
	Body           string                 `json:"body,omitempty"`
	Priority       int                    `json:"priority,omitempty"`
	Tags           map[string]interface{} `json:"tags,omitempty"`
	Resource       *Resource              `json:"resource,omitempty"`
}

// Resource structure of the SAP Alert Notification Service Event
type Resource struct {
	ResourceName     string                 `json:"resourceName,omitempty"`
	ResourceType     string                 `json:"resourceType,omitempty"`
	ResourceInstance string                 `json:"resourceInstance,omitempty"`
	Tags             map[string]interface{} `json:"tags,omitempty"`
}

// UnmarshallServiceKeyJSON unmarshalls the given json service key string.
func UnmarshallServiceKeyJSON(serviceKeyJSON string) (ansServiceKey ServiceKey, err error) {
	err = json.Unmarshal([]byte(serviceKeyJSON), &ansServiceKey)
	if err != nil {
		err = errors.Wrap(err, "error unmarshalling ANS serviceKey")
		return
	}
	return
}

// MergeWithJSON unmarshalls an ANS Event JSON string and merges it with the existing receiver Event
func (event *Event) MergeWithJSON(eventJSON []byte) (err error) {
	err = json.Unmarshal(eventJSON, &event)
	if err != nil {
		err = errors.Wrapf(err, "error unmarshalling ANS event from JSON string %q", eventJSON)
		return
	}
	return
}

// Send an event to the SAP Alert Notification Service
func (ans ANS) Send(event Event) error {
	const eventPath = "/cf/producer/v1/resource-events"

	requestBody, err := json.Marshal(event)
	if err != nil {
		return err
	}

	header := make(http.Header)
	err = ans.XSUAA.SetAuthHeaderIfNotPresent(&header)
	if err != nil {
		return err
	}

	entireUrl := ans.URL + eventPath

	httpClient := http.Client{}
	request, err := http.NewRequest(http.MethodPost, entireUrl, bytes.NewBuffer(requestBody))
	if err != nil {
		return err
	}
	request.Header.Add(authHeaderKey, header.Get(authHeaderKey))
	request.Header.Add("Content-Type", "application/json")
	response, err := httpClient.Do(request)
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusAccepted {
		statusCodeError := fmt.Errorf("ANS http request to '%s' failed. Did not get expected status code %d; instead got %d",
			entireUrl, http.StatusAccepted, response.StatusCode)
		responseBody, err := body.ReadResponseBody(response)
		if err != nil {
			err = errors.Wrapf(err, "%s; reading response body failed", statusCodeError.Error())
		} else {
			err = fmt.Errorf("%s; response body: %s", statusCodeError.Error(), responseBody)
		}
		return err
	}

	return nil
}

// TranslateLogrusLogLevel takes the logrus log level and translates it to an ANS severity ans category string
func TranslateLogrusLogLevel(level logrus.Level) (severity, category string) {
	severity = infoSeverity
	category = notificationCategory
	switch level {
	case logrus.InfoLevel:
		severity = infoSeverity
		category = notificationCategory
	case logrus.DebugLevel:
		severity = infoSeverity
		category = notificationCategory
	case logrus.WarnLevel:
		severity = warningSeverity
		category = alertCategory
	case logrus.ErrorLevel:
		severity = errorSeverity
		category = exceptionCategory
	case logrus.FatalLevel:
		severity = fatalSeverity
		category = exceptionCategory
	case logrus.PanicLevel:
		severity = fatalSeverity
		category = exceptionCategory
	}
	return
}
