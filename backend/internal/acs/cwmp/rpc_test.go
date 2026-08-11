package cwmp

import (
	"strings"
	"testing"
)

func TestSetParameterValues(t *testing.T) {
	payload := string(SetParameterValues(OutboundCommand{
		ID: "command-1", AttemptID: "attempt-1",
		Parameters: []ParameterValue{{Name: "Device.WiFi.SSID.1.SSID", Value: "A&B", Type: "string"}},
	}))
	for _, expected := range []string{"SetParameterValues", "attempt-1", "Device.WiFi.SSID.1.SSID", "A&amp;B"} {
		if !strings.Contains(payload, expected) {
			t.Fatalf("payload does not contain %q: %s", expected, payload)
		}
	}
}

func TestParseCommandResult(t *testing.T) {
	success := []byte(`<Envelope><Body><SetParameterValuesResponse><Status>0</Status></SetParameterValuesResponse></Body></Envelope>`)
	result, err := ParseCommandResult(success)
	if err != nil || result.Fault {
		t.Fatalf("success result = %+v, %v", result, err)
	}
	fault := []byte(`<Envelope><Body><Fault><faultcode>Client</faultcode><faultstring>CWMP fault</faultstring><detail><Fault><FaultCode>9007</FaultCode><FaultString>Invalid value</FaultString></Fault></detail></Fault></Body></Envelope>`)
	result, err = ParseCommandResult(fault)
	if err != nil || !result.Fault || result.FaultCode != "9007" {
		t.Fatalf("fault result = %+v, %v", result, err)
	}
}
