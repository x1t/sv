package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/x1t/sv/pkg/supervisor"
)

func TestRPCClientMapsSupervisorProcessInfo(t *testing.T) {
	server := newRPCServer(t, func(method string) string {
		if method != "supervisor.getAllProcessInfo" {
			return rpcFaultResponse("unexpected method")
		}
		return rpcProcessListResponse()
	})
	defer server.Close()

	processes, err := supervisor.NewRPCClient(server.URL, "", "").GetAllProcesses()
	require.NoError(t, err)
	require.Len(t, processes, 1)
	process := processes[0]
	assert.Equal(t, "web:api", process.Name)
	assert.Equal(t, "web", process.Group)
	assert.Equal(t, 20, process.State)
	assert.Equal(t, "RUNNING", process.StateName)
	assert.Equal(t, 1234, process.PID)
	assert.Equal(t, float64(1000), process.Start)
	assert.Equal(t, float64(0), process.Stop)
	assert.Equal(t, float64(3661), process.Now)
	assert.Empty(t, process.SpawnErr)
	assert.Equal(t, "/var/log/web.log", process.Logfile)
	assert.Equal(t, "/var/log/web.out", process.StdoutLogfile)
	assert.Equal(t, "/var/log/web.err", process.StderrLogfile)
	assert.Equal(t, "API process", process.Description)
	assert.Equal(t, "44分21秒", process.Uptime)
	assert.Equal(t, 0, process.ExitStatus)
}

func TestRPCClientControlsThroughXMLRPC(t *testing.T) {
	var methods []string
	server := newRPCServer(t, func(method string) string {
		methods = append(methods, method)
		return rpcBooleanResponse(true)
	})
	defer server.Close()

	client := supervisor.NewRPCClient(server.URL, "user", "password")
	require.NoError(t, client.ControlProcess("restart", "web:api"))
	assert.Equal(t, []string{"supervisor.stopProcess", "supervisor.startProcess"}, methods)
}

func TestRPCClientPreservesFalseBooleanResponse(t *testing.T) {
	server := newRPCServer(t, func(string) string { return rpcBooleanResponse(false) })
	defer server.Close()

	err := supervisor.NewRPCClient(server.URL, "", "").ControlProcess("start", "web:api")
	assert.ErrorContains(t, err, "拒绝")
}

func TestRPCClientRejectsInvalidEndpoint(t *testing.T) {
	_, err := supervisor.NewRPCClient("ftp://localhost/RPC2", "", "").GetAllProcesses()
	assert.ErrorContains(t, err, "http或https")
}

func TestRPCServerReceivesBasicAuthentication(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		username, password, ok := request.BasicAuth()
		if !ok || username != "user" || password != "password" {
			http.Error(writer, "missing authentication", http.StatusUnauthorized)
			return
		}
		writer.Header().Set("Content-Type", "text/xml")
		_, _ = io.WriteString(writer, rpcBooleanResponse(true))
	}))
	defer server.Close()

	require.NoError(t, supervisor.NewRPCClient(server.URL, "user", "password").ControlProcess("start", "web:api"))
}

func TestConfigDetectorDryRunAndAtomicUpdate(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "supervisord.conf")
	initial := "[supervisord]\nlogfile=/var/log/supervisord.log\n"
	require.NoError(t, os.WriteFile(configPath, []byte(initial), 0600))
	t.Setenv("SUPERVISOR_CONFIG", configPath)
	detector := supervisor.NewConfigDetector()

	message, err := detector.ConfigureRPC(true)
	require.NoError(t, err)
	assert.Contains(t, message, "将更新")
	unchanged, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Equal(t, initial, string(unchanged))

	_, err = detector.ConfigureRPC(false)
	require.NoError(t, err)
	updated, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(updated), "[inet_http_server]")
	assert.Contains(t, string(updated), "[rpcinterface:supervisor]")
	ok, err := detector.HasInetHTTPServer(configPath)
	require.NoError(t, err)
	assert.True(t, ok)
	ok, err = detector.HasRPCInterface(configPath)
	require.NoError(t, err)
	assert.True(t, ok)
	backups, err := filepath.Glob(configPath + ".bak.*")
	require.NoError(t, err)
	require.Len(t, backups, 1)

	message, err = detector.ConfigureRPC(false)
	require.NoError(t, err)
	assert.Contains(t, message, "已存在")
}

func newRPCServer(t *testing.T, response func(method string) string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		method := xmlMethodName(string(body))
		writer.Header().Set("Content-Type", "text/xml")
		_, _ = io.WriteString(writer, response(method))
	}))
}

func xmlMethodName(body string) string {
	start := strings.Index(body, "<methodName>")
	if start < 0 {
		return ""
	}
	start += len("<methodName>")
	end := strings.Index(body[start:], "</methodName>")
	if end < 0 {
		return ""
	}
	return body[start : start+end]
}

func rpcProcessListResponse() string {
	return `<?xml version="1.0"?><methodResponse><params><param><value><array><data><value><struct>
<member><name>name</name><value><string>api</string></value></member>
<member><name>group</name><value><string>web</string></value></member>
<member><name>start</name><value><double>1000</double></value></member>
<member><name>stop</name><value><double>0</double></value></member>
<member><name>now</name><value><double>3661</double></value></member>
<member><name>state</name><value><int>20</int></value></member>
<member><name>statename</name><value><string>RUNNING</string></value></member>
<member><name>spawnerr</name><value><string></string></value></member>
<member><name>exitstatus</name><value><int>0</int></value></member>
<member><name>logfile</name><value><string>/var/log/web.log</string></value></member>
<member><name>stdout_logfile</name><value><string>/var/log/web.out</string></value></member>
<member><name>stderr_logfile</name><value><string>/var/log/web.err</string></value></member>
<member><name>pid</name><value><int>1234</int></value></member>
<member><name>description</name><value><string>API process</string></value></member>
</struct></value></data></array></value></param></params></methodResponse>`
}

func rpcBooleanResponse(value bool) string {
	digit := "0"
	if value {
		digit = "1"
	}
	return fmt.Sprintf(`<?xml version="1.0"?><methodResponse><params><param><value><boolean>%s</boolean></value></param></params></methodResponse>`, digit)
}

func rpcFaultResponse(message string) string {
	escaped := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(message)
	return fmt.Sprintf(`<?xml version="1.0"?><methodResponse><fault><value><struct><member><name>faultString</name><value><string>%s</string></value></member></struct></value></fault></methodResponse>`, escaped)
}
