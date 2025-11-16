package supervisor

import "encoding/xml"

// XML-RPC数据结构
type MethodCall struct {
	XMLName    xml.Name   `xml:"methodCall"`
	MethodName string     `xml:"methodName"`
	Params     []Param    `xml:"params>param"`
}

type Param struct {
	Value Value `xml:"value"`
}

type Value struct {
	String  string      `xml:"string,omitempty"`
	Int     int         `xml:"int,omitempty"`
	Boolean bool        `xml:"boolean,omitempty"`
	Array   ArrayValues `xml:"array,omitempty"`
}

// ArrayValues 用于处理数组值
type ArrayValues struct {
	Data ArrayData `xml:"data"`
}

type ArrayData struct {
	Values []Value `xml:"value"`
}

type MethodResponse struct {
	XMLName xml.Name `xml:"methodResponse"`
	Params  []Param  `xml:"params>param"`
	Fault   *Fault   `xml:"fault"`
}

type Fault struct {
	Value struct {
		Struct struct {
			Member []struct {
				Name  string `xml:"name"`
				Value Value  `xml:"value"`
			} `xml:"member"`
		} `xml:"struct"`
	} `xml:"value"`
}

// EnhancedValue 用于更好地表示XML-RPC响应值
type EnhancedValue struct {
	XMLName xml.Name    `xml:"value"`
	String  string      `xml:"string"`
	Int     int         `xml:"int"`
	Boolean bool        `xml:"boolean"`
	Double  float64     `xml:"double"`
	Array   EnhancedArray `xml:"array"`
	Struct  EnhancedStruct `xml:"struct"`
}

// EnhancedArray 表示XML-RPC数组
type EnhancedArray struct {
	Data EnhancedData `xml:"data"`
}

// EnhancedData 包含数组的数据
type EnhancedData struct {
	Values []EnhancedValue `xml:"value"`
}

// EnhancedStruct 表示XML-RPC结构体
type EnhancedStruct struct {
	Members []EnhancedMember `xml:"member"`
}

// EnhancedMember 表示结构体成员
type EnhancedMember struct {
	Name  string        `xml:"name"`
	Value EnhancedValue `xml:"value"`
}

// ProcessInfoRPC 定义从RPC获取的进程信息结构
type ProcessInfoRPC struct {
	Name        string  `xml:"name"`
	Group       string  `xml:"group"`
	Start       float64 `xml:"start"`
	Stop        float64 `xml:"stop"`
	Now         float64 `xml:"now"`
	State       int     `xml:"state"`
	StateName   string  `xml:"statename"`
	SpawnErr    string  `xml:"spawnerr"`
	ExitStatus  int     `xml:"exitstatus"`
	Logfile     string  `xml:"logfile"`
	StdoutLogfile string `xml:"stdout_logfile"`
	StderrLogfile string `xml:"stderr_logfile"`
	Pid         int     `xml:"pid"`
	Description string  `xml:"description"`
}