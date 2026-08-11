package cwmp

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
)

var ErrInvalidInform = errors.New("invalid CWMP Inform")

type Inform struct {
	ID           string
	Manufacturer string
	OUI          string
	ProductClass string
	SerialNumber string
	Events       []string
	Parameters   map[string]string
}

type envelope struct {
	Header struct {
		ID string `xml:"ID"`
	} `xml:"Header"`
	Body struct {
		Inform *struct {
			DeviceID struct {
				Manufacturer string `xml:"Manufacturer"`
				OUI          string `xml:"OUI"`
				ProductClass string `xml:"ProductClass"`
				SerialNumber string `xml:"SerialNumber"`
			} `xml:"DeviceId"`
			Events []struct {
				Code string `xml:"EventCode"`
			} `xml:"Event>EventStruct"`
			Parameters []struct {
				Name  string `xml:"Name"`
				Value string `xml:"Value"`
			} `xml:"ParameterList>ParameterValueStruct"`
		} `xml:"Inform"`
	} `xml:"Body"`
}

func ParseInform(reader io.Reader) (Inform, error) {
	var message envelope
	decoder := xml.NewDecoder(reader)
	if err := decoder.Decode(&message); err != nil {
		return Inform{}, fmt.Errorf("%w: malformed XML", ErrInvalidInform)
	}
	if message.Body.Inform == nil {
		return Inform{}, fmt.Errorf("%w: body is not Inform", ErrInvalidInform)
	}
	device := message.Body.Inform.DeviceID
	result := Inform{
		ID: strings.TrimSpace(message.Header.ID), Manufacturer: strings.TrimSpace(device.Manufacturer),
		OUI: strings.ToUpper(strings.TrimSpace(device.OUI)), ProductClass: strings.TrimSpace(device.ProductClass),
		SerialNumber: strings.TrimSpace(device.SerialNumber), Parameters: make(map[string]string),
	}
	if result.OUI == "" || result.SerialNumber == "" {
		return Inform{}, fmt.Errorf("%w: OUI and SerialNumber are required", ErrInvalidInform)
	}
	for _, event := range message.Body.Inform.Events {
		if code := strings.TrimSpace(event.Code); code != "" {
			result.Events = append(result.Events, code)
		}
	}
	for _, parameter := range message.Body.Inform.Parameters {
		if name := strings.TrimSpace(parameter.Name); name != "" {
			result.Parameters[name] = strings.TrimSpace(parameter.Value)
		}
	}
	return result, nil
}

func Parameter(inform Inform, suffix string) string {
	for name, value := range inform.Parameters {
		if strings.HasSuffix(name, suffix) {
			return value
		}
	}
	return ""
}
