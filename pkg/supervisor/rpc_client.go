package supervisor

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/x1t/sv/pkg/utils"
)

const (
	DefaultSupervisorHost = "http://localhost:9001/RPC2"
	defaultHTTPTimeout    = 10 * time.Second
	defaultCommandTimeout = 10 * time.Second
	// defaultControlTimeout 是 start/stop/restart 等同步控制操作的超时。这类调用会
	// 一直阻塞到 supervisor 处理完优雅停止(stopwaitsecs)与启动判定(startsecs),
	// 慢服务常超过 10s,故单独给更长默认值,可用 SUPERVISOR_TIMEOUT 覆盖。
	defaultControlTimeout = 120 * time.Second
	maxRPCResponseSize    = 16 << 20
)

// ControlTimeoutEnv 是覆盖同步控制操作超时的环境变量名,取值单位秒。
const ControlTimeoutEnv = "SUPERVISOR_TIMEOUT"

// RPCClient is a client for Supervisor's XML-RPC endpoint.
type RPCClient struct {
	host             string
	username         string
	password         string
	client           *http.Client // 查询用,默认 defaultHTTPTimeout
	opClient         *http.Client // 控制操作用,默认 defaultControlTimeout
	commandPath      string
	commandTimeout   time.Duration // 查询命令回退超时
	opCommandTimeout time.Duration // 控制命令回退超时
}

// NewRPCClient creates a Supervisor client. An empty host uses the local
// Supervisor endpoint.
func NewRPCClient(host, username, password string) *RPCClient {
	if strings.TrimSpace(host) == "" {
		host = DefaultSupervisorHost
	}
	controlTimeout := resolveControlTimeout()

	return &RPCClient{
		host:             host,
		username:         username,
		password:         password,
		client:           &http.Client{Timeout: defaultHTTPTimeout, CheckRedirect: refuseRedirects},
		opClient:         &http.Client{Timeout: controlTimeout, CheckRedirect: refuseRedirects},
		commandPath:      "supervisorctl",
		commandTimeout:   defaultCommandTimeout,
		opCommandTimeout: controlTimeout,
	}
}

// resolveControlTimeout 解析 SUPERVISOR_TIMEOUT(单位秒)作为同步控制操作的超时;
// 缺省、非法或小于 1 时回退到 defaultControlTimeout。
func resolveControlTimeout() time.Duration {
	raw := strings.TrimSpace(os.Getenv(ControlTimeoutEnv))
	if raw == "" {
		return defaultControlTimeout
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds < 1 {
		return defaultControlTimeout
	}
	return time.Duration(seconds) * time.Second
}

func refuseRedirects(_ *http.Request, _ []*http.Request) error {
	return http.ErrUseLastResponse
}

// controlClient 返回控制专用连接;对直接以结构体字面量构造、未初始化 opClient 的
// 场景做兜底,避免 nil 解引用。
func (rc *RPCClient) controlClient() *http.Client {
	if rc.opClient != nil {
		return rc.opClient
	}
	return &http.Client{Timeout: resolveControlTimeout(), CheckRedirect: refuseRedirects}
}

// call invokes one XML-RPC method. It is kept as a small wrapper so callers
// that do not need cancellation can use the default context.
func (rc *RPCClient) call(method string, params []interface{}) (interface{}, error) {
	return rc.callContext(context.Background(), method, params)
}

// callContext 用于只读查询,使用默认(10s)连接。
func (rc *RPCClient) callContext(ctx context.Context, method string, params []interface{}) (interface{}, error) {
	return rc.callContextWithClient(ctx, rc.client, method, params)
}

// callControlContext 用于 start/stop/restart 等同步控制操作,使用控制专用的长超时
// 连接,避免慢启动/慢停止服务在默认 10s 内被误判为失败。
func (rc *RPCClient) callControlContext(ctx context.Context, method string, params []interface{}) (interface{}, error) {
	return rc.callContextWithClient(ctx, rc.controlClient(), method, params)
}

func (rc *RPCClient) callContextWithClient(ctx context.Context, client *http.Client, method string, params []interface{}) (interface{}, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(method) == "" {
		return nil, fmt.Errorf("XML-RPC方法名不能为空")
	}
	if err := validateEndpoint(rc.host); err != nil {
		return nil, err
	}

	call := MethodCall{MethodName: method}
	for _, param := range params {
		value, err := newRPCValue(param)
		if err != nil {
			return nil, fmt.Errorf("XML-RPC参数无效: %w", err)
		}
		call.Params = append(call.Params, Param{Value: value})
	}

	xmlData, err := xml.Marshal(call)
	if err != nil {
		return nil, fmt.Errorf("XML序列化失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rc.host, bytes.NewReader(xmlData))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Accept", "text/xml")
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("User-Agent", "sv-supervisor-client/1.0")
	if rc.username != "" {
		req.SetBasicAuth(rc.username, rc.password)
	}

	if client == nil {
		client = &http.Client{Timeout: defaultHTTPTimeout, CheckRedirect: refuseRedirects}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求Supervisor失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxRPCResponseSize+1))
	if err != nil {
		return nil, fmt.Errorf("读取Supervisor响应失败: %w", err)
	}
	if len(body) > maxRPCResponseSize {
		return nil, fmt.Errorf("Supervisor响应超过%d字节限制", maxRPCResponseSize)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Supervisor返回HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var response MethodResponse
	if err := xml.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("XML解析失败: %w", err)
	}
	if response.Fault != nil {
		return nil, formatRPCFault(response.Fault)
	}
	if len(response.Params) == 0 {
		return nil, nil
	}

	value, err := response.Params[0].Value.toInterface()
	if err != nil {
		return nil, fmt.Errorf("XML-RPC响应值无效: %w", err)
	}
	return value, nil
}

func validateEndpoint(rawURL string) error {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return fmt.Errorf("Supervisor地址无效: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("Supervisor地址必须使用http或https")
	}
	if u.Host == "" || u.User != nil {
		return fmt.Errorf("Supervisor地址必须包含主机且不能在URL中嵌入认证信息")
	}
	return nil
}

func newRPCValue(input interface{}) (Value, error) {
	switch value := input.(type) {
	case nil:
		return Value{Nil: &struct{}{}}, nil
	case string:
		return Value{String: stringPointer(value)}, nil
	case bool:
		boolean := RPCBoolean(value)
		return Value{Boolean: &boolean}, nil
	case int:
		number := int64(value)
		return Value{Int: &number}, nil
	case int8:
		number := int64(value)
		return Value{Int: &number}, nil
	case int16:
		number := int64(value)
		return Value{Int: &number}, nil
	case int32:
		number := int64(value)
		return Value{Int: &number}, nil
	case int64:
		number := value
		return Value{Int: &number}, nil
	case uint:
		if uint64(value) > math.MaxInt64 {
			return Value{}, fmt.Errorf("整数超出XML-RPC范围")
		}
		number := int64(value)
		return Value{Int: &number}, nil
	case uint8:
		number := int64(value)
		return Value{Int: &number}, nil
	case uint16:
		number := int64(value)
		return Value{Int: &number}, nil
	case uint32:
		number := int64(value)
		return Value{Int: &number}, nil
	case uint64:
		if value > math.MaxInt64 {
			return Value{}, fmt.Errorf("整数超出XML-RPC范围")
		}
		number := int64(value)
		return Value{Int: &number}, nil
	case float32:
		number := float64(value)
		return Value{Double: &number}, nil
	case float64:
		number := value
		return Value{Double: &number}, nil
	case []string:
		values := make([]interface{}, len(value))
		for i, item := range value {
			values[i] = item
		}
		return newRPCValue(values)
	case []interface{}:
		values := make([]Value, len(value))
		for i, item := range value {
			parsed, err := newRPCValue(item)
			if err != nil {
				return Value{}, err
			}
			values[i] = parsed
		}
		return Value{Array: &ArrayValues{Data: ArrayData{Values: values}}}, nil
	case map[string]interface{}:
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		members := make([]StructMember, 0, len(keys))
		for _, key := range keys {
			parsed, err := newRPCValue(value[key])
			if err != nil {
				return Value{}, err
			}
			members = append(members, StructMember{Name: key, Value: parsed})
		}
		return Value{Struct: &StructValues{Members: members}}, nil
	default:
		return Value{}, fmt.Errorf("不支持的参数类型 %T", input)
	}
}

func stringPointer(value string) *string {
	return &value
}

func (value Value) toInterface() (interface{}, error) {
	switch {
	case value.String != nil:
		return *value.String, nil
	case value.Int != nil:
		return *value.Int, nil
	case value.I4 != nil:
		return *value.I4, nil
	case value.I8 != nil:
		return *value.I8, nil
	case value.Boolean != nil:
		return bool(*value.Boolean), nil
	case value.Double != nil:
		return *value.Double, nil
	case value.Array != nil:
		result := make([]interface{}, len(value.Array.Data.Values))
		for i, item := range value.Array.Data.Values {
			parsed, err := item.toInterface()
			if err != nil {
				return nil, err
			}
			result[i] = parsed
		}
		return result, nil
	case value.Struct != nil:
		result := make(map[string]interface{}, len(value.Struct.Members))
		for _, member := range value.Struct.Members {
			parsed, err := member.Value.toInterface()
			if err != nil {
				return nil, err
			}
			result[member.Name] = parsed
		}
		return result, nil
	case value.Nil != nil:
		return nil, nil
	default:
		return nil, fmt.Errorf("空或未知的XML-RPC值")
	}
}

func formatRPCFault(fault *Fault) error {
	value, err := fault.Value.toInterface()
	if err == nil {
		if members, ok := value.(map[string]interface{}); ok {
			if message, ok := members["faultString"].(string); ok && message != "" {
				return fmt.Errorf("XML-RPC错误: %s", message)
			}
		}
		return fmt.Errorf("XML-RPC错误: %v", value)
	}
	return fmt.Errorf("XML-RPC错误: %w", err)
}

// GetAllProcesses obtains all process information, falling back to the local
// supervisorctl command only for the default local endpoint.
func (rc *RPCClient) GetAllProcesses() ([]utils.ProcessInfo, error) {
	return rc.GetAllProcessesContext(context.Background())
}

func (rc *RPCClient) GetAllProcessesContext(ctx context.Context) ([]utils.ProcessInfo, error) {
	result, err := rc.callContext(ctx, "supervisor.getAllProcessInfo", nil)
	if err != nil {
		if rc.canUseCommandFallback() {
			processes, fallbackErr := rc.getAllProcessesViaCommand(ctx)
			if fallbackErr == nil {
				return processes, nil
			}
			return nil, fmt.Errorf("RPC获取进程失败: %v；supervisorctl回退失败: %w", err, fallbackErr)
		}
		return nil, fmt.Errorf("RPC获取进程失败: %w", err)
	}

	processesData, ok := result.([]interface{})
	if !ok {
		return nil, fmt.Errorf("RPC返回的进程列表类型错误: %T", result)
	}

	processes := make([]utils.ProcessInfo, 0, len(processesData))
	for _, processData := range processesData {
		processMap, ok := processData.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("RPC进程信息类型错误: %T", processData)
		}
		process, err := parseProcessInfoFromMap(processMap, len(processes)+1)
		if err != nil {
			return nil, err
		}
		processes = append(processes, process)
	}
	return processes, nil
}

func parseProcessInfoFromMap(processMap map[string]interface{}, index int) (utils.ProcessInfo, error) {
	name, ok := stringValue(processMap, "name")
	if !ok || strings.TrimSpace(name) == "" {
		return utils.ProcessInfo{}, fmt.Errorf("RPC进程缺少有效名称")
	}
	group, _ := stringValue(processMap, "group")
	start, _ := floatValue(processMap["start"])
	stop, _ := floatValue(processMap["stop"])
	now, _ := floatValue(processMap["now"])
	state, _ := intValue(processMap["state"])
	stateName, _ := stringValue(processMap, "statename")
	spawnErr, _ := stringValue(processMap, "spawnerr")
	pid, _ := intValue(processMap["pid"])
	logfile, _ := stringValue(processMap, "logfile")
	stdoutLogfile, _ := stringValue(processMap, "stdout_logfile")
	stderrLogfile, _ := stringValue(processMap, "stderr_logfile")
	exitStatus, _ := intValue(processMap["exitstatus"])
	description, _ := stringValue(processMap, "description")

	fullName := name
	if group != "" && group != name && name != "" && !strings.Contains(name, ":") {
		fullName = group + ":" + name
	}

	uptime := ""
	if state == utils.StateStopped || pid == 0 {
		uptime = "已停止"
	} else if start > 0 && now >= start {
		uptime = utils.FormatUptime(int(now - start))
	}

	return utils.ProcessInfo{
		Index:         index,
		Name:          fullName,
		Group:         group,
		Start:         start,
		Stop:          stop,
		Now:           now,
		State:         state,
		StateName:     stateName,
		SpawnErr:      spawnErr,
		PID:           pid,
		Logfile:       logfile,
		StdoutLogfile: stdoutLogfile,
		StderrLogfile: stderrLogfile,
		Uptime:        uptime,
		Description:   description,
		ExitStatus:    exitStatus,
	}, nil
}

func stringValue(values map[string]interface{}, key string) (string, bool) {
	value, ok := values[key].(string)
	return value, ok
}

func intValue(value interface{}) (int, bool) {
	var number int64
	switch typed := value.(type) {
	case int:
		return typed, true
	case int8:
		return int(typed), true
	case int16:
		return int(typed), true
	case int32:
		return int(typed), true
	case int64:
		number = typed
	case uint:
		if uint64(typed) > uint64(maxInt()) {
			return 0, false
		}
		return int(typed), true
	case uint8:
		return int(typed), true
	case uint16:
		return int(typed), true
	case uint32:
		if uint64(typed) > uint64(maxInt()) {
			return 0, false
		}
		return int(typed), true
	case uint64:
		if typed > uint64(maxInt()) {
			return 0, false
		}
		return int(typed), true
	case float32:
		if float32(math.Trunc(float64(typed))) != typed || float64(typed) < float64(minInt()) || float64(typed) > float64(maxInt()) {
			return 0, false
		}
		return int(typed), true
	case float64:
		if math.Trunc(typed) != typed || typed < float64(minInt()) || typed > float64(maxInt()) {
			return 0, false
		}
		return int(typed), true
	default:
		return 0, false
	}
	if number < int64(minInt()) || number > int64(maxInt()) {
		return 0, false
	}
	return int(number), true
}

func floatValue(value interface{}) (float64, bool) {
	switch typed := value.(type) {
	case float32:
		return float64(typed), true
	case float64:
		return typed, true
	case int:
		return float64(typed), true
	case int8:
		return float64(typed), true
	case int16:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	default:
		return 0, false
	}
}

func maxInt() int {
	return int(^uint(0) >> 1)
}

func minInt() int {
	return -maxInt() - 1
}

func (rc *RPCClient) canUseCommandFallback() bool {
	return strings.TrimRight(rc.host, "/") == strings.TrimRight(DefaultSupervisorHost, "/") && rc.username == "" && rc.password == ""
}

func (rc *RPCClient) getAllProcessesViaCommand(ctx context.Context) ([]utils.ProcessInfo, error) {
	output, err := rc.runSupervisorctl(ctx, "status")
	processes := utils.ParseSupervisorctlOutput(string(output))
	if err != nil && len(processes) == 0 {
		return nil, fmt.Errorf("supervisorctl status失败: %w; 输出: %s", err, strings.TrimSpace(string(output)))
	}
	if len(processes) == 0 && strings.TrimSpace(string(output)) == "" {
		return nil, fmt.Errorf("supervisorctl未返回进程信息")
	}
	return processes, nil
}

// runSupervisorctl 用于只读查询(status),使用查询命令超时。
func (rc *RPCClient) runSupervisorctl(ctx context.Context, args ...string) ([]byte, error) {
	return rc.runSupervisorctlWith(ctx, rc.commandTimeout, defaultCommandTimeout, args...)
}

// runSupervisorctlControl 用于 start/stop/restart 命令回退,使用控制超时。
func (rc *RPCClient) runSupervisorctlControl(ctx context.Context, args ...string) ([]byte, error) {
	return rc.runSupervisorctlWith(ctx, rc.opCommandTimeout, defaultControlTimeout, args...)
}

func (rc *RPCClient) runSupervisorctlWith(ctx context.Context, timeout, fallback time.Duration, args ...string) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if timeout <= 0 {
		timeout = fallback
	}
	commandContext, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	commandPath := rc.commandPath
	if commandPath == "" {
		commandPath = "supervisorctl"
	}
	return exec.CommandContext(commandContext, commandPath, args...).CombinedOutput()
}

// ControlProcess uses XML-RPC for local and remote control. Local fallback is
// limited to the default endpoint so a remote failure can never affect a local
// Supervisor accidentally.
func (rc *RPCClient) ControlProcess(action, processName string) error {
	action = strings.ToLower(strings.TrimSpace(action))
	if err := validateProcessAction(action); err != nil {
		return err
	}
	if err := validateProcessName(processName); err != nil {
		return err
	}

	ctx := context.Background()
	if action == "restart" {
		// 目标本来就未在运行(含已停止/FATAL)时,stop 会报 NOT_RUNNING:跳过停止
		// 阶段直接启动,与 supervisorctl restart 的语义保持一致。
		if err := rc.controlProcessRPC(ctx, "supervisor.stopProcess", processName); err != nil && !isNotRunningError(err) {
			if rc.canUseCommandFallback() {
				return rc.controlProcessViaCommand(ctx, action, processName)
			}
			return fmt.Errorf("重启进程失败（停止阶段）: %w", err)
		}
		if err := rc.controlProcessRPC(ctx, "supervisor.startProcess", processName); err != nil {
			return fmt.Errorf("重启进程失败（启动阶段）: %w", err)
		}
		return nil
	}

	method := "supervisor." + action + "Process"
	if err := rc.controlProcessRPC(ctx, method, processName); err == nil {
		return nil
	} else if !rc.canUseCommandFallback() {
		return fmt.Errorf("%s进程失败: %w", action, err)
	}
	return rc.controlProcessViaCommand(ctx, action, processName)
}

// isNotRunningError 判断错误是否为 supervisor 的 NOT_RUNNING(目标本来未在运行)。
func isNotRunningError(err error) bool {
	return err != nil && strings.Contains(strings.ToUpper(err.Error()), "NOT_RUNNING")
}

func (rc *RPCClient) controlProcessRPC(ctx context.Context, method, processName string) error {
	result, err := rc.callControlContext(ctx, method, []interface{}{processName, true})
	if err != nil {
		return err
	}
	if success, ok := result.(bool); ok && !success {
		return fmt.Errorf("Supervisor拒绝了操作")
	}
	return nil
}

func (rc *RPCClient) controlProcessViaCommand(ctx context.Context, action, processName string) error {
	output, err := rc.runSupervisorctlControl(ctx, action, processName)
	if err != nil {
		return fmt.Errorf("%s进程失败: %w, 输出: %s", action, err, strings.TrimSpace(string(output)))
	}
	if strings.Contains(strings.ToUpper(string(output)), "ERROR") {
		return fmt.Errorf("%s进程失败: %s", action, strings.TrimSpace(string(output)))
	}
	return nil
}
