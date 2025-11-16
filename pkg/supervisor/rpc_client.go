package supervisor

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"
	"sv/pkg/utils"
)

// RPCClient Supervisor RPC客户端
type RPCClient struct {
	host     string
	username string
	password string
	client   *http.Client
}

// NewRPCClient 创建新的Supervisor客户端
func NewRPCClient(host, username, password string) *RPCClient {
	return &RPCClient{
		host:     host,
		username: username,
		password: password,
		client: &http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// 禁止HTTP重定向以防止SSRF攻击
				return http.ErrUseLastResponse
			},
		},
	}
}

// call 调用XML-RPC方法
func (rc *RPCClient) call(method string, params []interface{}) (interface{}, error) {
	// 构建methodCall
	call := MethodCall{
		MethodName: method,
	}

	if params != nil {
		for _, param := range params {
			var value Value
			switch v := param.(type) {
			case string:
				value.String = v
			case int:
				value.Int = v
			case bool:
				value.Boolean = v
			case []interface{}:  // 处理数组参数
				value.Array = ArrayValues{
					Data: ArrayData{
						Values: make([]Value, len(v)),
					},
				}
				for i, item := range v {
					switch iv := item.(type) {
					case string:
						value.Array.Data.Values[i] = Value{String: iv}
					case int:
						value.Array.Data.Values[i] = Value{Int: iv}
					case bool:
						value.Array.Data.Values[i] = Value{Boolean: iv}
					}
				}
			}
			call.Params = append(call.Params, Param{Value: value})
		}
	}

	// 序列化为XML
	xmlData, err := xml.Marshal(call)
	if err != nil {
		return nil, fmt.Errorf("XML序列化失败: %v", err)
	}

	// 创建HTTP请求
	req, err := http.NewRequest("POST", rc.host, bytes.NewBuffer(xmlData))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}

	req.Header.Set("Content-Type", "text/xml")
	req.Header.Set("User-Agent", "sv-supervisor-client/1.0")

	// 添加认证
	if rc.username != "" && rc.password != "" {
		req.SetBasicAuth(rc.username, rc.password)
	}

	// 发送请求
	resp, err := rc.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}

	// 确保在所有路径下都关闭响应体
	defer func() {
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
	}()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP错误: %d, %s", resp.StatusCode, string(body))
	}

	// 为了正确解析响应，我们需要使用EnhancedValue结构
	// 重新定义MethodResponse使用EnhancedValue
	response := struct {
		XMLName xml.Name      `xml:"methodResponse"`
		Params  []struct {
			Value EnhancedValue `xml:"param>value"`
		} `xml:"params"`
		Fault *struct {
			Value struct {
				Struct struct {
					Member []struct {
						Name  string        `xml:"name"`
						Value EnhancedValue `xml:"value"`
					} `xml:"member"`
				} `xml:"struct"`
			} `xml:"value"`
		} `xml:"fault"`
	}{}

	if err := xml.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("XML解析失败: %v", err)
	}

	// 检查错误
	if response.Fault != nil {
		for _, member := range response.Fault.Value.Struct.Member {
			if member.Name == "faultString" {
				return nil, fmt.Errorf("XML-RPC错误: %s", member.Value.String)
			}
		}
		return nil, fmt.Errorf("未知XML-RPC错误")
	}

	if len(response.Params) == 0 {
		return nil, nil
	}

	// 解析并返回数据
	return rc.parseEnhancedValue(response.Params[0].Value), nil
}

// parseEnhancedValue 将EnhancedValue转换为Go类型
func (rc *RPCClient) parseEnhancedValue(ev EnhancedValue) interface{} {
	if ev.String != "" {
		return ev.String
	}
	if ev.Int != 0 || (ev.String == "" && !ev.Boolean && ev.Double == 0 && ev.Array.Data.Values == nil && ev.Struct.Members == nil) {
		// 如果int不是0，或者这是唯一设置的字段，则返回int
		return ev.Int
	}
	if ev.Boolean {
		return ev.Boolean
	}
	if ev.Double != 0 {
		return ev.Double
	}
	if ev.Array.Data.Values != nil {
		// 解析数组
		result := make([]interface{}, len(ev.Array.Data.Values))
		for i, val := range ev.Array.Data.Values {
			result[i] = rc.parseEnhancedValue(val)
		}
		return result
	}
	if ev.Struct.Members != nil {
		// 解析结构体
		result := make(map[string]interface{})
		for _, member := range ev.Struct.Members {
			result[member.Name] = rc.parseEnhancedValue(member.Value)
		}
		return result
	}
	return nil
}

// GetAllProcesses 获取所有进程信息
func (rc *RPCClient) GetAllProcesses() ([]utils.ProcessInfo, error) {
	// 首先尝试使用RPC调用
	result, err := rc.call("supervisor.getAllProcessInfo", nil)
	if err != nil {
		// 如果RPC调用失败，回退到使用命令行方式
		fmt.Printf("⚠️  RPC调用失败: %v, 尝试使用命令行工具\n", err)
		return rc.getAllProcessesViaCommand()
	}

	// 将结果转换为适当的类型
	if processesData, ok := result.([]interface{}); ok {
		processes := make([]utils.ProcessInfo, len(processesData))
		for i, procData := range processesData {
			if procMap, ok := procData.(map[string]interface{}); ok {
				processes[i] = rc.parseProcessInfoFromMap(procMap, i+1)
			}
		}
		return processes, nil
	}

	fmt.Println("⚠️  无法解析RPC响应数据，使用命令行工具作为回退")
	return rc.getAllProcessesViaCommand()
}

// parseProcessInfoFromMap 从map解析进程信息
func (rc *RPCClient) parseProcessInfoFromMap(procMap map[string]interface{}, index int) utils.ProcessInfo {
	name := ""
	if n, ok := procMap["name"]; ok && n != nil {
		if s, ok := n.(string); ok {
			name = s
		}
	}

	group := ""
	if g, ok := procMap["group"]; ok && g != nil {
		if s, ok := g.(string); ok {
			group = s
		}
	}

	state := 0
	if s, ok := procMap["state"]; ok && s != nil {
		if f, ok := s.(float64); ok {
			state = int(f)
		} else if i, ok := s.(int); ok {
			state = i
		} else if i, ok := s.(float32); ok {
			state = int(i)
		}
	}

	stateName := ""
	if sn, ok := procMap["statename"]; ok && sn != nil {
		if s, ok := sn.(string); ok {
			stateName = s
		}
	}

	pid := 0
	if p, ok := procMap["pid"]; ok && p != nil {
		if f, ok := p.(float64); ok {
			pid = int(f)
		} else if i, ok := p.(int); ok {
			pid = i
		} else if i, ok := p.(float32); ok {
			pid = int(i)
		}
	}

	description := ""
	if d, ok := procMap["description"]; ok && d != nil {
		if s, ok := d.(string); ok {
			description = s
		}
	}

	// 生成完整进程名称 (group:name)
	fullName := name
	if group != "" && name != "" {
		fullName = group + ":" + name
	}

	// 生成状态描述
	var uptime string
	if pid > 0 {
		// 如果有PID，尝试从描述中提取运行时间
		uptime = description
	} else {
		uptime = "已停止"
	}

	return utils.ProcessInfo{
		Index:       index,
		Name:        fullName,  // 使用完整进程名称
		Group:       group,
		State:       state,
		StateName:   stateName,
		PID:         pid,
		Uptime:      uptime,
		Description: utils.GetStateIcon(state),
		ExitStatus:  0,
	}
}

// getAllProcessesViaCommand 通过命令行方式获取进程信息（回退方案）
func (rc *RPCClient) getAllProcessesViaCommand() ([]utils.ProcessInfo, error) {
	// 尝试使用 supervisorctl 命令获取真实数据
	fmt.Println("正在获取Supervisor进程状态...")
	cmd := exec.Command("supervisorctl", "status")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// 即使有错误，output中通常也包含有用的信息
		outputStr := string(output)
		if strings.Contains(outputStr, "RUNNING") || strings.Contains(outputStr, "STOPPED") {
			fmt.Println("⚠️  获取到进程数据，但可能存在一些状态问题")
			return utils.ParseSupervisorctlOutput(outputStr), nil
		}
		fmt.Printf("❌ supervisorctl 命令失败: %v, 输出: %s\n", err, string(output))
		return nil, fmt.Errorf("无法获取进程信息: supervisorctl 命令失败: %v", err)
	}

	fmt.Println("✅ 成功获取真实进程数据")
	return utils.ParseSupervisorctlOutput(string(output)), nil
}