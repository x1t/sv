package supervisor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// sysvInitTemplate 是 SysV init 脚本模板(name/exe 以占位符标出)。
// 内容取自 sv-rs `sysv_init_text` 的实际输出(逐字节一致,case 体无缩进)。
const sysvInitTemplate = `#!/bin/sh
### BEGIN INIT INFO
# Provides:          sv-supervisor-manager
# Required-Start:    $network
# Required-Stop:     $network
# Default-Start:     2 3 4 5
# Default-Stop:      0 1 6
# Short-Description: SV Supervisor Manager
### END INIT INFO

name=__NAME__
exe=__EXE__
pidfile=/run/$name.pid

case "$1" in
start)
if [ -f "$pidfile" ] && kill -0 "$(cat "$pidfile" 2>/dev/null)" 2>/dev/null; then
exit 0
fi
"$exe" daemon >>/var/log/$name.log 2>&1 &
echo $! > "$pidfile"
;;
stop)
[ -f "$pidfile" ] || exit 0
pid="$(cat "$pidfile" 2>/dev/null)" || exit 1
if ! kill -0 "$pid" 2>/dev/null; then
rm -f "$pidfile"
exit 0
fi
kill "$pid" 2>/dev/null || exit 1
for i in $(seq 1 10)
do
if ! kill -0 "$pid" 2>/dev/null; then
rm -f "$pidfile"
exit 0
fi
sleep 1
done
echo "not stopped" >&2
exit 1
;;
restart)
$0 stop
$0 start
;;
status)
if [ -f "$pidfile" ] && kill -0 "$(cat "$pidfile" 2>/dev/null)" 2>/dev/null; then
echo "running"
exit 0
fi
echo "not running"
exit 3
;;
*)
echo "Usage: $0 {start|stop|restart|status}" >&2
exit 1
;;
esac
`

// ServiceBackend 标识当前 Linux 服务管理后端(镜像 sv-rs `ServiceBackend`)。
type ServiceBackend int

const (
	// BackendSystemd 使用 systemd(unit 文件 + systemctl)。
	BackendSystemd ServiceBackend = iota
	// BackendSysV 使用传统 SysV(/etc/init.d 脚本 + rc*.d 链接)。
	BackendSysV
)

func (b ServiceBackend) String() string {
	if b == BackendSystemd {
		return "systemd"
	}
	return "sysv"
}

// detectBackend 探测后端:systemd 活跃且 systemctl 可用时为 Systemd,否则 SysV。
func detectBackend() ServiceBackend {
	systemdRunning := false
	if _, err := os.Stat("/run/systemd/system"); err == nil {
		systemdRunning = true
	} else if data, err := os.ReadFile("/proc/1/comm"); err == nil {
		systemdRunning = strings.TrimSpace(string(data)) == "systemd"
	}
	if systemdRunning {
		if _, err := exec.LookPath("systemctl"); err == nil {
			return BackendSystemd
		}
	}
	return BackendSysV
}

// systemdEscapeExecutable 把可执行路径里的空格写作 \x20,避免被当作参数分隔。
func systemdEscapeExecutable(executable string) string {
	return strings.ReplaceAll(executable, " ", `\x20`)
}

// shellQuote 用单引号包裹并转义 ' 为 '\”,供 SysV init 脚本内嵌路径使用。
func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// systemdUnitText 生成 systemd unit 文件内容(与 sv-rs `systemd_unit_text` 逐字节一致)。
func systemdUnitText(executable string) string {
	return fmt.Sprintf(
		"[Unit]\nDescription=SV Supervisor Manager\nAfter=network.target\n\n"+
			"[Service]\nType=simple\nExecStart=%s daemon\nRestart=always\n\n"+
			"[Install]\nWantedBy=multi-user.target\n",
		systemdEscapeExecutable(executable),
	)
}

// sysvInitText 生成 SysV init.d 脚本内容(与 sv-rs `sysv_init_text` 逐字节一致)。
func sysvInitText(executable string) string {
	replacer := strings.NewReplacer(
		"__NAME__", serviceName,
		"__EXE__", shellQuote(executable),
	)
	return replacer.Replace(sysvInitTemplate)
}

// systemdUnitExecutable 从 unit 文本解析回 ExecStart 可执行路径(\x20 还原空格)。
// 只接受无前缀绝对路径的首段;解析不到返回空串。
func systemdUnitExecutable(text string) string {
	for _, rawLine := range strings.Split(text, "\n") {
		line := strings.TrimRight(rawLine, " \t")
		if !strings.HasPrefix(line, "ExecStart=") {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(line, "ExecStart="))
		if rest == "" || rest[0] != '/' {
			return ""
		}
		first := strings.Fields(rest)[0]
		return strings.ReplaceAll(first, `\x20`, " ")
	}
	return ""
}

// sysvInitExecutable 从 init 脚本文本解析回 exe 记录的单引号值(反转 shellQuote)。
func sysvInitExecutable(text string) string {
	for _, rawLine := range strings.Split(text, "\n") {
		line := strings.TrimSpace(rawLine)
		if !strings.HasPrefix(line, "exe=") {
			continue
		}
		value := strings.TrimPrefix(line, "exe=")
		if !strings.HasPrefix(value, "'") || !strings.HasSuffix(value, "'") || len(value) < 2 {
			return ""
		}
		unquoted := strings.ReplaceAll(value[1:len(value)-1], `'\''`, "'")
		if unquoted == "" {
			return ""
		}
		return unquoted
	}
	return ""
}

// runCommandResult 是一次子进程执行的合并输出与退出码。
type runCommandResult struct {
	output []byte
	code   int
}

// runCommandWithTimeout 运行命令并等待结束,超过 timeout 杀死命令进程。
// 返回约定:
//   - err != nil:命令无法启动(如二进制不存在)或执行超时;
//   - err == nil:进程已退出,code 为退出码(0 表示成功,非零为普通失败)。
//
// 区分「启动失败」与「正常退出非零」很关键:前者应报错,后者按退出码处理,
// 否则「命令不存在」会被误判成 code=-1 的“信号终止”。
func runCommandWithTimeout(bin string, args []string, timeout time.Duration) (runCommandResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	var buffer bytes.Buffer
	cmd.Stdout = &buffer
	cmd.Stderr = &buffer
	// CommandContext 超时默认只 kill 命令进程;对 sh -c 这类会 fork 子孙的情况,
	// 显式设置 WaitDelay 并关闭 stdio 管道,确保超时后尽快回收。
	cmd.WaitDelay = 100 * time.Millisecond
	err := cmd.Run()
	if err != nil {
		// 超时优先:context 到期时 CommandContext 会杀掉进程,err 可能是 ExitError(-1),
		// 应先识别超时再按退出码处理。
		if ctx.Err() == context.DeadlineExceeded {
			return runCommandResult{buffer.Bytes(), -1}, fmt.Errorf("命令执行超时(%s): %s %s", timeout, bin, strings.Join(args, " "))
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// 进程确实运行并退出非零:按退出码返回,不作为“启动失败”。
			return runCommandResult{buffer.Bytes(), exitErr.ExitCode()}, nil
		}
		// 启动失败(命令不存在、无权限等)。
		return runCommandResult{buffer.Bytes(), -1}, fmt.Errorf("执行命令失败: %s %s: %w", bin, strings.Join(args, " "), err)
	}
	return runCommandResult{buffer.Bytes(), 0}, nil
}

// waitForSignal 阻塞直到收到 SIGTERM/SIGINT;供 daemon 优雅退出(镜像 sv-rs)。
func waitForSignal() {
	channel := make(chan os.Signal, 1)
	signal.Notify(channel, syscall.SIGTERM, syscall.SIGINT)
	<-channel
}
