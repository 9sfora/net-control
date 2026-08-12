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

func TestDownload(t *testing.T) {
	payload := string(Download(OutboundCommand{ID: "command-1", AttemptID: "attempt-1", Type: "UPDATE_FIRMWARE", Download: &DownloadRequest{URL: "https://acs.example/firmware?a=1&b=2", FileSize: 42, FileName: "router.bin"}}))
	for _, expected := range []string{"cwmp:Download", "command-1", "attempt-1", "1 Firmware Upgrade Image", "https://acs.example/firmware?a=1&amp;b=2", "<FileSize>42</FileSize>", "router.bin"} {
		if !strings.Contains(payload, expected) {
			t.Fatalf("payload does not contain %q: %s", expected, payload)
		}
	}
}

func TestParseTransferComplete(t *testing.T) {
	result, err := ParseCommandResult([]byte(`<Envelope><Body><TransferComplete><CommandKey>command-1</CommandKey><FaultStruct><FaultCode>0</FaultCode><FaultString></FaultString></FaultStruct></TransferComplete></Body></Envelope>`))
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind != "TRANSFER_COMPLETE" || result.CommandKey != "command-1" || result.Fault {
		t.Fatalf("unexpected result: %+v", result)
	}
}
