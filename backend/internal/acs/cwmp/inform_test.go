package cwmp

import (
	"errors"
	"strings"
	"testing"
)

const sampleInform = `<?xml version="1.0"?>
<soap-env:Envelope xmlns:soap-env="http://schemas.xmlsoap.org/soap/envelope/" xmlns:cwmp="urn:dslforum-org:cwmp-1-2">
  <soap-env:Header><cwmp:ID soap-env:mustUnderstand="1">42</cwmp:ID></soap-env:Header>
  <soap-env:Body><cwmp:Inform>
    <DeviceId><Manufacturer>Net Router</Manufacturer><OUI>a1b2c3</OUI><ProductClass>NR-1</ProductClass><SerialNumber>SN123</SerialNumber></DeviceId>
    <Event><EventStruct><EventCode>1 BOOT</EventCode><CommandKey></CommandKey></EventStruct></Event>
    <ParameterList soap-enc:arrayType="cwmp:ParameterValueStruct[1]" xmlns:soap-enc="http://schemas.xmlsoap.org/soap/encoding/">
      <ParameterValueStruct><Name>Device.DeviceInfo.SoftwareVersion</Name><Value>1.2.3</Value></ParameterValueStruct>
    </ParameterList>
  </cwmp:Inform></soap-env:Body>
</soap-env:Envelope>`

func TestParseInform(t *testing.T) {
	inform, err := ParseInform(strings.NewReader(sampleInform))
	if err != nil {
		t.Fatalf("ParseInform() error = %v", err)
	}
	if inform.ID != "42" || inform.OUI != "A1B2C3" || inform.SerialNumber != "SN123" {
		t.Fatalf("unexpected identity: %+v", inform)
	}
	if len(inform.Events) != 1 || Parameter(inform, ".DeviceInfo.SoftwareVersion") != "1.2.3" {
		t.Fatalf("unexpected Inform data: %+v", inform)
	}
}

func TestParseInformRejectsMissingIdentity(t *testing.T) {
	_, err := ParseInform(strings.NewReader(`<Envelope><Body><Inform><DeviceId/></Inform></Body></Envelope>`))
	if !errors.Is(err, ErrInvalidInform) {
		t.Fatalf("error = %v, want ErrInvalidInform", err)
	}
}
