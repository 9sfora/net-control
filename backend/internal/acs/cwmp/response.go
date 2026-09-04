package cwmp

import (
	"bytes"
	"encoding/xml"
)

func InformResponse(id string) []byte {
	var escaped bytes.Buffer
	_ = xml.EscapeText(&escaped, []byte(id))
	return []byte(`<?xml version="1.0" encoding="UTF-8"?>` +
		`<soap-env:Envelope xmlns:soap-env="http://schemas.xmlsoap.org/soap/envelope/" xmlns:cwmp="urn:dslforum-org:cwmp-1-2">` +
		`<soap-env:Header><cwmp:ID soap-env:mustUnderstand="1">` + escaped.String() + `</cwmp:ID></soap-env:Header>` +
		`<soap-env:Body><cwmp:InformResponse><MaxEnvelopes>1</MaxEnvelopes></cwmp:InformResponse></soap-env:Body>` +
		`</soap-env:Envelope>`)
}

func TransferCompleteResponse() []byte {
	return []byte(`<?xml version="1.0" encoding="UTF-8"?>` +
		`<soap-env:Envelope xmlns:soap-env="http://schemas.xmlsoap.org/soap/envelope/" xmlns:cwmp="urn:dslforum-org:cwmp-1-0">` +
		`<soap-env:Body><cwmp:TransferCompleteResponse/></soap-env:Body>` +
		`</soap-env:Envelope>`)
}
