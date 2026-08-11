package cwmp

import (
	"bytes"
	"encoding/xml"
	"fmt"
)

type ParameterValue struct {
	Name  string
	Value string
	Type  string
}

type OutboundCommand struct {
	ID         string
	AttemptID  string
	Type       string
	Parameters []ParameterValue
}

type CommandResult struct {
	Fault       bool
	FaultCode   string
	FaultString string
	Raw         string
}

func SetParameterValues(command OutboundCommand) []byte {
	var body bytes.Buffer
	body.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	body.WriteString(`<soap-env:Envelope xmlns:soap-env="http://schemas.xmlsoap.org/soap/envelope/" xmlns:soap-enc="http://schemas.xmlsoap.org/soap/encoding/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xsd="http://www.w3.org/2001/XMLSchema" xmlns:cwmp="urn:dslforum-org:cwmp-1-2">`)
	body.WriteString(`<soap-env:Header><cwmp:ID soap-env:mustUnderstand="1">`)
	escape(&body, command.AttemptID)
	body.WriteString(`</cwmp:ID></soap-env:Header><soap-env:Body><cwmp:SetParameterValues><ParameterList soap-enc:arrayType="cwmp:ParameterValueStruct[`)
	body.WriteString(fmt.Sprintf("%d", len(command.Parameters)))
	body.WriteString(`]">`)
	for _, parameter := range command.Parameters {
		body.WriteString(`<ParameterValueStruct><Name>`)
		escape(&body, parameter.Name)
		body.WriteString(`</Name><Value xsi:type="xsd:`)
		escape(&body, parameter.Type)
		body.WriteString(`">`)
		escape(&body, parameter.Value)
		body.WriteString(`</Value></ParameterValueStruct>`)
	}
	body.WriteString(`</ParameterList><ParameterKey>`)
	escape(&body, command.ID)
	body.WriteString(`</ParameterKey></cwmp:SetParameterValues></soap-env:Body></soap-env:Envelope>`)
	return body.Bytes()
}

func ParseCommandResult(payload []byte) (CommandResult, error) {
	var message struct {
		Body struct {
			Response *struct{} `xml:"SetParameterValuesResponse"`
			Fault    *struct {
				Code   string `xml:"faultcode"`
				String string `xml:"faultstring"`
				Detail struct {
					Fault struct {
						Code   string `xml:"FaultCode"`
						String string `xml:"FaultString"`
					} `xml:"Fault"`
				} `xml:"detail"`
			} `xml:"Fault"`
		} `xml:"Body"`
	}
	if err := xml.Unmarshal(payload, &message); err != nil {
		return CommandResult{}, err
	}
	result := CommandResult{Raw: string(payload)}
	if message.Body.Response != nil {
		return result, nil
	}
	if message.Body.Fault != nil {
		result.Fault = true
		result.FaultCode = message.Body.Fault.Detail.Fault.Code
		result.FaultString = message.Body.Fault.Detail.Fault.String
		if result.FaultCode == "" {
			result.FaultCode = message.Body.Fault.Code
			result.FaultString = message.Body.Fault.String
		}
		return result, nil
	}
	return CommandResult{}, fmt.Errorf("unsupported CWMP response")
}

func escape(buffer *bytes.Buffer, value string) { _ = xml.EscapeText(buffer, []byte(value)) }
