package supervisor

import (
	"encoding/xml"
	"fmt"
	"strings"
)

// MethodCall describes an XML-RPC request.
type MethodCall struct {
	XMLName    xml.Name `xml:"methodCall"`
	MethodName string   `xml:"methodName"`
	Params     []Param  `xml:"params>param"`
}

// Param wraps one XML-RPC value.
type Param struct {
	Value Value `xml:"value"`
}

// Value represents the XML-RPC scalar and composite types used by Supervisor.
// Pointers are intentional: XML-RPC must distinguish false, zero and empty
// string from a missing value.
type Value struct {
	String  *string       `xml:"string"`
	Int     *int64        `xml:"int"`
	I4      *int64        `xml:"i4"`
	I8      *int64        `xml:"i8"`
	Boolean *RPCBoolean   `xml:"boolean"`
	Double  *float64      `xml:"double"`
	Array   *ArrayValues  `xml:"array"`
	Struct  *StructValues `xml:"struct"`
	Nil     *struct{}     `xml:"nil"`
}

// ArrayValues represents an XML-RPC array.
type ArrayValues struct {
	Data ArrayData `xml:"data"`
}

type ArrayData struct {
	Values []Value `xml:"value"`
}

// StructValues represents an XML-RPC struct.
type StructValues struct {
	Members []StructMember `xml:"member"`
}

type StructMember struct {
	Name  string `xml:"name"`
	Value Value  `xml:"value"`
}

// RPCBoolean accepts both XML-RPC's 0/1 representation and true/false.
type RPCBoolean bool

func (b *RPCBoolean) UnmarshalXML(decoder *xml.Decoder, start xml.StartElement) error {
	var raw string
	if err := decoder.DecodeElement(&raw, &start); err != nil {
		return err
	}

	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true":
		*b = true
	case "0", "false":
		*b = false
	default:
		return fmt.Errorf("无效的XML-RPC布尔值: %q", raw)
	}
	return nil
}

// MarshalXML 按 XML-RPC 规范输出 1/0。Supervisor 的布尔解析器只认 0/1,
// 若用 encoding/xml 默认的 true/false 文本会被当作假值,导致 wait 参数失效。
func (b RPCBoolean) MarshalXML(encoder *xml.Encoder, start xml.StartElement) error {
	digit := "0"
	if b {
		digit = "1"
	}
	return encoder.EncodeElement(digit, start)
}

// MethodResponse describes an XML-RPC response from Supervisor.
type MethodResponse struct {
	XMLName xml.Name `xml:"methodResponse"`
	Params  []Param  `xml:"params>param"`
	Fault   *Fault   `xml:"fault"`
}

type Fault struct {
	Value Value `xml:"value"`
}

// ProcessInfoRPC contains the fields returned by supervisor.getAllProcessInfo.
type ProcessInfoRPC struct {
	Name          string  `xml:"name"`
	Group         string  `xml:"group"`
	Start         float64 `xml:"start"`
	Stop          float64 `xml:"stop"`
	Now           float64 `xml:"now"`
	State         int     `xml:"state"`
	StateName     string  `xml:"statename"`
	SpawnErr      string  `xml:"spawnerr"`
	ExitStatus    int     `xml:"exitstatus"`
	Logfile       string  `xml:"logfile"`
	StdoutLogfile string  `xml:"stdout_logfile"`
	StderrLogfile string  `xml:"stderr_logfile"`
	PID           int     `xml:"pid"`
	Description   string  `xml:"description"`
}
