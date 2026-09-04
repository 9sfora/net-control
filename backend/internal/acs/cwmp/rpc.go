package cwmp

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"
)

type ParameterValue struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Type  string `json:"type"`
}

type OutboundCommand struct {
	ID         string
	AttemptID  string
	Type       string
	Parameters []ParameterValue
	Download   *DownloadRequest
}

type DownloadRequest struct {
	URL      string
	FileSize int64
	FileName string
}

type CommandResult struct {
	Kind           string
	CommandKey     string
	TransferCode   string
	ResponseStatus string
	Fault          bool
	FaultCode      string
	FaultString    string
	Raw            string
	Parameters     []ParameterValue
}

func EncodeCommand(command OutboundCommand) []byte {
	if command.Type == "UPDATE_FIRMWARE" {
		return Download(command)
	}
	if command.Type == "SYNC" {
		return GetParameterValues(command)
	}
	if command.Type == "REBOOT" || command.Type == "FACTORY_RESET" {
		return SimpleRPC(command)
	}
	return SetParameterValues(command)
}

func SimpleRPC(command OutboundCommand) []byte {
	method, content := "Reboot", "<CommandKey>"+parameterKey(command.ID)+"</CommandKey>"
	if command.Type == "FACTORY_RESET" {
		method, content = "FactoryReset", ""
	}
	var body bytes.Buffer
	body.WriteString(`<?xml version="1.0" encoding="UTF-8"?><soap-env:Envelope xmlns:soap-env="http://schemas.xmlsoap.org/soap/envelope/" xmlns:cwmp="urn:dslforum-org:cwmp-1-2"><soap-env:Header><cwmp:ID soap-env:mustUnderstand="1">`)
	escape(&body, command.AttemptID)
	body.WriteString(`</cwmp:ID></soap-env:Header><soap-env:Body><cwmp:` + method + `>` + content + `</cwmp:` + method + `></soap-env:Body></soap-env:Envelope>`)
	return body.Bytes()
}

func GetParameterValues(command OutboundCommand) []byte {
	var body bytes.Buffer
	body.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	body.WriteString(`<soap-env:Envelope xmlns:soap-env="http://schemas.xmlsoap.org/soap/envelope/" xmlns:soap-enc="http://schemas.xmlsoap.org/soap/encoding/" xmlns:cwmp="urn:dslforum-org:cwmp-1-2"><soap-env:Header><cwmp:ID soap-env:mustUnderstand="1">`)
	escape(&body, command.AttemptID)
	body.WriteString(`</cwmp:ID></soap-env:Header><soap-env:Body><cwmp:GetParameterValues><ParameterNames soap-enc:arrayType="xsd:string[`)
	body.WriteString(fmt.Sprintf("%d", len(command.Parameters)))
	body.WriteString(`]" xmlns:xsd="http://www.w3.org/2001/XMLSchema">`)
	for _, parameter := range command.Parameters {
		body.WriteString(`<string>`)
		escape(&body, parameter.Name)
		body.WriteString(`</string>`)
	}
	body.WriteString(`</ParameterNames></cwmp:GetParameterValues></soap-env:Body></soap-env:Envelope>`)
	return body.Bytes()
}

func Download(command OutboundCommand) []byte {
	var body bytes.Buffer
	body.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	body.WriteString(`<soap-env:Envelope xmlns:soap-env="http://schemas.xmlsoap.org/soap/envelope/" xmlns:cwmp="urn:dslforum-org:cwmp-1-2"><soap-env:Header><cwmp:ID soap-env:mustUnderstand="1">`)
	escape(&body, command.AttemptID)
	body.WriteString(`</cwmp:ID></soap-env:Header><soap-env:Body><cwmp:Download><CommandKey>`)
	escape(&body, parameterKey(command.ID))
	body.WriteString(`</CommandKey><FileType>1 Firmware Upgrade Image</FileType><URL>`)
	escape(&body, command.Download.URL)
	body.WriteString(`</URL><Username></Username><Password></Password><FileSize>`)
	body.WriteString(fmt.Sprintf("%d", command.Download.FileSize))
	body.WriteString(`</FileSize><TargetFileName>`)
	escape(&body, command.Download.FileName)
	body.WriteString(`</TargetFileName><DelaySeconds>0</DelaySeconds><SuccessURL></SuccessURL><FailureURL></FailureURL></cwmp:Download></soap-env:Body></soap-env:Envelope>`)
	return body.Bytes()
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
	escape(&body, parameterKey(command.ID))
	body.WriteString(`</ParameterKey></cwmp:SetParameterValues></soap-env:Body></soap-env:Envelope>`)
	return body.Bytes()
}

func ParseCommandResult(payload []byte) (CommandResult, error) {
	var message struct {
		Body struct {
			Response    *struct{} `xml:"SetParameterValuesResponse"`
			GetResponse *struct {
				Parameters []struct {
					Name  string `xml:"Name"`
					Value struct {
						Type  string `xml:"type,attr"`
						Value string `xml:",chardata"`
					} `xml:"Value"`
				} `xml:"ParameterList>ParameterValueStruct"`
			} `xml:"GetParameterValuesResponse"`
			RebootResponse       *struct{} `xml:"RebootResponse"`
			FactoryResetResponse *struct{} `xml:"FactoryResetResponse"`
			DownloadResponse     *struct {
				Status string `xml:"Status"`
			} `xml:"DownloadResponse"`
			TransferComplete *struct {
				CommandKey string `xml:"CommandKey"`
				Fault      struct {
					Code   string `xml:"FaultCode"`
					String string `xml:"FaultString"`
				} `xml:"FaultStruct"`
			} `xml:"TransferComplete"`
			Fault *struct {
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
		result.Kind = "SET_PARAMETER_VALUES"
		return result, nil
	}
	if message.Body.GetResponse != nil {
		result.Kind = "GET_PARAMETER_VALUES"
		result.Parameters = make([]ParameterValue, 0, len(message.Body.GetResponse.Parameters))
		for _, parameter := range message.Body.GetResponse.Parameters {
			valueType := parameter.Value.Type
			if separator := strings.Index(valueType, ":"); separator >= 0 {
				valueType = valueType[separator+1:]
			}
			result.Parameters = append(result.Parameters, ParameterValue{Name: parameter.Name, Value: parameter.Value.Value, Type: valueType})
		}
		return result, nil
	}
	if message.Body.RebootResponse != nil {
		result.Kind = "REBOOT"
		return result, nil
	}
	if message.Body.FactoryResetResponse != nil {
		result.Kind = "FACTORY_RESET"
		return result, nil
	}
	if message.Body.DownloadResponse != nil {
		result.Kind = "DOWNLOAD_ACCEPTED"
		result.ResponseStatus = message.Body.DownloadResponse.Status
		return result, nil
	}
	if message.Body.TransferComplete != nil {
		result.Kind = "TRANSFER_COMPLETE"
		result.CommandKey = message.Body.TransferComplete.CommandKey
		result.TransferCode = message.Body.TransferComplete.Fault.Code
		result.FaultString = message.Body.TransferComplete.Fault.String
		result.Fault = result.TransferCode != "" && result.TransferCode != "0"
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

func parameterKey(commandID string) string {
	key := strings.ReplaceAll(commandID, "-", "")
	if len(key) > 32 {
		return key[:32]
	}
	return key
}
