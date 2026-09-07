package supervisor

import (
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveControlTimeoutDefaultsToLongTimeout(t *testing.T) {
	t.Setenv(ControlTimeoutEnv, "")
	assert.Equal(t, defaultControlTimeout, resolveControlTimeout())

	t.Setenv(ControlTimeoutEnv, "abc")
	assert.Equal(t, defaultControlTimeout, resolveControlTimeout())

	t.Setenv(ControlTimeoutEnv, "0")
	assert.Equal(t, defaultControlTimeout, resolveControlTimeout())

	t.Setenv(ControlTimeoutEnv, "-5")
	assert.Equal(t, defaultControlTimeout, resolveControlTimeout())
}

func TestResolveControlTimeoutReadsEnvironment(t *testing.T) {
	t.Setenv(ControlTimeoutEnv, "300")
	assert.Equal(t, 300*time.Second, resolveControlTimeout())

	t.Setenv(ControlTimeoutEnv, " 45 ")
	assert.Equal(t, 45*time.Second, resolveControlTimeout())
}

func TestNewRPCClientSplitsQueryAndControlTimeouts(t *testing.T) {
	t.Setenv(ControlTimeoutEnv, "5")
	client := NewRPCClient("", "", "")

	assert.Equal(t, defaultHTTPTimeout, client.client.Timeout)
	assert.Equal(t, 5*time.Second, client.opClient.Timeout)
	assert.Equal(t, defaultCommandTimeout, client.commandTimeout)
	assert.Equal(t, 5*time.Second, client.opCommandTimeout)
}

// TestControlProcessUsesOpClientTimeout 通过把查询连接调成比控制连接更小来验证
// start/stop/restart 走的是控制专用连接:若误走查询连接会在 40ms 内超时失败,
// 走控制连接(3s)则能等完服务端 250ms 延迟后成功。
func TestControlProcessUsesOpClientTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		time.Sleep(250 * time.Millisecond)
		writer.Header().Set("Content-Type", "text/xml")
		_, _ = writer.Write([]byte(rpcBooleanXML(true)))
	}))
	defer server.Close()

	client := &RPCClient{
		host:           server.URL,
		client:         &http.Client{Timeout: 40 * time.Millisecond},
		opClient:       &http.Client{Timeout: 3 * time.Second},
		commandPath:    "supervisorctl",
		commandTimeout: time.Second,
	}
	require.NoError(t, client.ControlProcess("start", "web:api"))
}

// TestControlProcessDoesNotFallBackToQueryClient 反向校验:控制连接过小、查询连接
// 足够大时 start 会超时失败,证明控制流量没有悄悄退回到查询连接。
func TestControlProcessDoesNotFallBackToQueryClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		time.Sleep(250 * time.Millisecond)
		writer.Header().Set("Content-Type", "text/xml")
		_, _ = writer.Write([]byte(rpcBooleanXML(true)))
	}))
	defer server.Close()

	client := &RPCClient{
		host:           server.URL,
		client:         &http.Client{Timeout: 3 * time.Second},
		opClient:       &http.Client{Timeout: 40 * time.Millisecond},
		commandPath:    "supervisorctl",
		commandTimeout: time.Second,
	}
	err := client.ControlProcess("start", "web:api")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "请求Supervisor失败")
}

// TestBooleanParamEncodesAsOneOrZero 保证请求里的布尔按 XML-RPC 规范输出 1/0:
// 若输出 true/false 文本,supervisor 会把它当假值,wait 参数将静默失效。
func TestBooleanParamEncodesAsOneOrZero(t *testing.T) {
	for _, tc := range []struct {
		want bool
		text string
	}{{true, "1"}, {false, "0"}} {
		value, err := newRPCValue(tc.want)
		require.NoError(t, err)
		call := MethodCall{MethodName: "supervisor.startProcess"}
		call.Params = append(call.Params, Param{Value: value})

		data, err := xml.Marshal(call)
		require.NoError(t, err)
		assert.Contains(t, string(data), "<boolean>"+tc.text+"</boolean>")
		assert.NotContains(t, string(data), "true</boolean>")
		assert.NotContains(t, string(data), "false</boolean>")
	}
}

// TestRestartSkipsNotRunningStop 停止阶段返回 NOT_RUNNING(进程本来就未运行)时,
// restart 应跳过停止、继续启动,与 supervisorctl restart 语义一致。
func TestRestartSkipsNotRunningStop(t *testing.T) {
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		method := "startProcess"
		if strings.Contains(string(body), "stopProcess") {
			method = "stopProcess"
		}
		methods = append(methods, method)
		writer.Header().Set("Content-Type", "text/xml")
		if method == "stopProcess" {
			_, _ = writer.Write([]byte(rpcFaultXML("NOT_RUNNING: web:api")))
			return
		}
		_, _ = writer.Write([]byte(rpcBooleanXML(true)))
	}))
	defer server.Close()

	client := NewRPCClient(server.URL, "", "")
	require.NoError(t, client.ControlProcess("restart", "web:api"))
	assert.Equal(t, []string{"stopProcess", "startProcess"}, methods)
}

// TestRestartHardStopErrorFails 停止阶段遇到非 NOT_RUNNING 的硬错误时,restart 应失败。
func TestRestartHardStopErrorFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/xml")
		_, _ = writer.Write([]byte(rpcFaultXML("ALREADY_STARTED: web:api")))
	}))
	defer server.Close()

	client := NewRPCClient(server.URL, "", "")
	err := client.ControlProcess("restart", "web:api")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "重启进程失败（停止阶段）")
}

func rpcFaultXML(message string) string {
	return `<?xml version="1.0"?><methodResponse><fault><value><struct><member><name>faultString</name><value><string>` + message + `</string></value></member></struct></value></fault></methodResponse>`
}

func rpcBooleanXML(value bool) string {
	digit := "0"
	if value {
		digit = "1"
	}
	return `<?xml version="1.0"?><methodResponse><params><param><value><boolean>` + digit + `</boolean></value></param></params></methodResponse>`
}
