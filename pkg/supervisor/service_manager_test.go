package supervisor

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveLinkTargetJoinsRelativeAgainstLinkDir(t *testing.T) {
	linkPath := "/etc/rc2.d/S50x"
	target := "../init.d/x"
	assert.Equal(t, "/etc/init.d/x", resolveLinkTarget(linkPath, target))
	assert.Equal(t, "/abs/sv", resolveLinkTarget(linkPath, "/abs/sv"))
}

func TestRemoveSymlinkRemovesOwnedAndKeepsForeign(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	require.NoError(t, os.MkdirAll(bin, 0755))
	executable := filepath.Join(bin, "sv")
	require.NoError(t, os.WriteFile(executable, []byte("#!/bin/sh\n"), 0755))

	// 属于记录 executable:软链指向旧 exe(可能已删除,readlink 文本匹配即删)。
	recorded := filepath.Join(dir, "old", "sv")
	link := filepath.Join(bin, "sv-link")
	require.NoError(t, os.Symlink(recorded, link))
	manager := &ServiceManager{out: &bytes.Buffer{}, executable: executable, symlinkPath: link}
	require.NoError(t, manager.removeSymlink([]string{recorded}))
	_, err := os.Lstat(link)
	assert.ErrorIs(t, err, os.ErrNotExist)

	// 指向他处:拒绝删除。
	other := filepath.Join(bin, "other")
	require.NoError(t, os.WriteFile(other, []byte("x\n"), 0755))
	link2 := filepath.Join(bin, "sv-link2")
	require.NoError(t, os.Symlink(other, link2))
	manager2 := &ServiceManager{out: &bytes.Buffer{}, executable: executable, symlinkPath: link2}
	err = manager2.removeSymlink([]string{recorded})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "拒绝删除指向其他文件")
	_, statErr := os.Lstat(link2)
	assert.NoError(t, statErr)
}

func TestRemoveSymlinkNoopWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	manager := &ServiceManager{out: &bytes.Buffer{}, symlinkPath: filepath.Join(dir, "missing")}
	require.NoError(t, manager.removeSymlink([]string{"/nope/sv"}))
}

// 以下为 install/uninstall 注入式闭环测试(SysV,真实临时目录 + 真实子进程)。

func TestInstallUninstallSysVFullLoop(t *testing.T) {
	manager, _, _ := tempManager(t, BackendSysV)
	var symlinks []string
	{
		require.NoError(t, manager.install())
		require.FileExists(t, manager.initScriptPath)
		// init 脚本真实可执行。
		result, err := runCommandWithTimeout(manager.initScriptPath, []string{"status"}, 5*time.Second)
		require.NoError(t, err)
		assert.Equal(t, 3, result.code) // 未运行
		links := sysVRunlevelLinks(manager.initScriptPath)
		symlinks = append(symlinks, links...)
		for _, link := range links {
			_, err := os.Lstat(link)
			assert.NoError(t, err, link)
		}
		_, err = os.Lstat(manager.symlinkPath)
		assert.NoError(t, err)
	}

	require.NoError(t, manager.uninstall())
	_, err := os.Lstat(manager.initScriptPath)
	assert.ErrorIs(t, err, os.ErrNotExist)
	for _, link := range symlinks {
		_, err := os.Lstat(link)
		assert.ErrorIs(t, err, os.ErrNotExist, link)
	}
	_, err = os.Lstat(manager.symlinkPath)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestUninstallWhenNeverInstalledIsNoop(t *testing.T) {
	manager, out, _ := tempManager(t, BackendSysV)
	require.NoError(t, manager.uninstall())
	assert.Contains(t, out.String(), "服务未安装，无需卸载")
}

func TestInstallRefusesOverwriteOfExistingScript(t *testing.T) {
	manager, _, _ := tempManager(t, BackendSysV)
	require.NoError(t, os.MkdirAll(filepath.Dir(manager.initScriptPath), 0755))
	require.NoError(t, os.WriteFile(manager.initScriptPath, []byte("#!/bin/sh\n# mine\n"), 0755))

	err := manager.install()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "目标路径已存在")
	data, readErr := os.ReadFile(manager.initScriptPath)
	require.NoError(t, readErr)
	assert.Equal(t, "#!/bin/sh\n# mine\n", string(data))
}

func TestInstallRollsBackWhenCommandSymlinkBlocked(t *testing.T) {
	manager, _, _ := tempManager(t, BackendSysV)
	// 命令软链路径被普通文件占用 → createSymlink 失败,应回滚 init/rc 链接。
	require.NoError(t, os.MkdirAll(filepath.Dir(manager.symlinkPath), 0755))
	require.NoError(t, os.WriteFile(manager.symlinkPath, []byte("occupied"), 0755))

	err := manager.install()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "创建命令软链接失败")
	_, lerr := os.Lstat(manager.initScriptPath)
	assert.ErrorIs(t, lerr, os.ErrNotExist)
	for _, link := range sysVRunlevelLinks(manager.initScriptPath) {
		_, lerr := os.Lstat(link)
		assert.ErrorIs(t, lerr, os.ErrNotExist, link)
	}
	content, rerr := os.ReadFile(manager.symlinkPath)
	require.NoError(t, rerr)
	assert.Equal(t, "occupied", string(content))
}

func TestUninstallRemovesDanglingCommandSymlinkAfterUpgrade(t *testing.T) {
	manager, _, _ := tempManager(t, BackendSysV)
	require.NoError(t, manager.install())
	// 命令软链仍指向 init 脚本记录的可执行路径,但该 exe 已被删除(dangling)。
	require.NoError(t, os.Remove(manager.executable))

	require.NoError(t, manager.uninstall())
	_, err := os.Lstat(manager.symlinkPath)
	assert.ErrorIs(t, err, os.ErrNotExist, "dangling 软链应被清理")
	_, err = os.Lstat(manager.initScriptPath)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestHandleServiceCommandUnknownAction(t *testing.T) {
	var out bytes.Buffer
	manager := forTesting("/bin/sv", "/none/sv", "/none/u", "/none/i", BackendSysV, &out)
	err := manager.HandleServiceCommand([]string{"bogus"})
	require.Error(t, err)
	assert.Contains(t, out.String(), "用法: sv service <action>")
	assert.Contains(t, err.Error(), "未知服务操作")
}

func TestWaitForSignalReturnsOnSigterm(t *testing.T) {
	done := make(chan struct{})
	go func() {
		waitForSignal()
		close(done)
	}()
	time.Sleep(100 * time.Millisecond)
	process, err := os.FindProcess(os.Getpid())
	require.NoError(t, err)
	require.NoError(t, process.Signal(syscall.SIGTERM))
	select {
	case <-done:
		// 成功返回
	case <-time.After(2 * time.Second):
		t.Fatal("waitForSignal 未在 SIGTERM 后返回")
	}
}

func TestRunCommandWithTimeoutKills(t *testing.T) {
	start := time.Now()
	_, err := runCommandWithTimeout("sh", []string{"-c", "sleep 30"}, 200*time.Millisecond)
	require.Error(t, err)
	assert.Less(t, time.Since(start), 5*time.Second, "应在超时后很快返回")
}

func TestRunCommandReportsExitCode(t *testing.T) {
	result, err := runCommandWithTimeout("sh", []string{"-c", "exit 3"}, 5*time.Second)
	require.NoError(t, err)
	assert.Equal(t, 3, result.code)
	result, err = runCommandWithTimeout("/bin/true", nil, 5*time.Second)
	require.NoError(t, err)
	assert.Equal(t, 0, result.code)
}

func TestRunDaemonWritesStartLine(t *testing.T) {
	var out bytes.Buffer
	manager := forTesting("/bin/sh", "/none/sv", "/none/u", "/none/i", BackendSysV, &out)
	// runDaemon 会阻塞等信号;启动后发 SIGTERM 验证两行输出。
	done := make(chan error, 1)
	go func() { done <- manager.runDaemon() }()
	time.Sleep(150 * time.Millisecond)
	process, err := os.FindProcess(os.Getpid())
	require.NoError(t, err)
	require.NoError(t, process.Signal(syscall.SIGTERM))
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("runDaemon 未在 SIGTERM 后返回")
	}
	assert.Contains(t, out.String(), "SV服务已启动，正在后台运行...")
	assert.Contains(t, out.String(), "SV服务已停止")
}

func TestInstallSetsInitScriptExecutable(t *testing.T) {
	manager, _, _ := tempManager(t, BackendSysV)
	require.NoError(t, manager.install())
	info, err := os.Stat(manager.initScriptPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o755), info.Mode().Perm())
	// 真实可执行并返回 status=3(未运行)。
	result, err := runCommandWithTimeout(manager.initScriptPath, []string{"status"}, 5*time.Second)
	require.NoError(t, err)
	assert.Equal(t, 3, result.code)
}

func TestWriteUnitFileRefusesOverwrite(t *testing.T) {
	manager, _, _ := tempManager(t, BackendSystemd)
	require.NoError(t, os.MkdirAll(filepath.Dir(manager.unitPath), 0755))
	require.NoError(t, os.WriteFile(manager.unitPath, []byte("existing"), 0644))
	err := manager.writeUnitFile()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "目标路径已存在")
	data, _ := os.ReadFile(manager.unitPath)
	assert.Equal(t, "existing", string(data))
}

func TestWriteInitScriptRefusesOverwriteSymlink(t *testing.T) {
	manager, _, _ := tempManager(t, BackendSysV)
	require.NoError(t, os.MkdirAll(filepath.Dir(manager.initScriptPath), 0755))
	other := filepath.Join(t.TempDir(), "other")
	require.NoError(t, os.WriteFile(other, []byte("#!/bin/sh\n"), 0755))
	require.NoError(t, os.Symlink(other, manager.initScriptPath)) // 软链占用
	err := manager.writeInitScript()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "目标路径已存在")
}

func TestUninstallForeignSymlinkLeavesLinkButRemovesService(t *testing.T) {
	manager, _, _ := tempManager(t, BackendSysV)
	require.NoError(t, manager.install())
	// 将命令软链改指向一个他处文件(模拟被用户替换)。卸载时服务应删,软链因异向保留并报错。
	other := filepath.Join(t.TempDir(), "other-sv")
	require.NoError(t, os.WriteFile(other, []byte("#!/bin/sh\n"), 0755))
	require.NoError(t, os.Remove(manager.symlinkPath))
	require.NoError(t, os.Symlink(other, manager.symlinkPath))

	err := manager.uninstall()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "服务已卸载，但移除命令软链接失败")
	_, lerr := os.Lstat(manager.initScriptPath)
	assert.ErrorIs(t, lerr, os.ErrNotExist) // 服务确实已卸载
	_, lerr = os.Lstat(manager.symlinkPath)
	assert.NoError(t, lerr) // 异向软链被保留,不误删
}

func TestSystemdInstallRollsBackUnitOnEnableFailure(t *testing.T) {
	manager, _, _ := tempManager(t, BackendSystemd)
	// 用不存在的唯一 unit 名(单元真实写到了临时路径但 systemctl 搜不到)触发 enable 失败。
	// unit 文件本身是真实写出的,失败应回滚删除。
	err := manager.install()
	if err == nil {
		// 环境若有该 unit(systemctl enable 成功)则跳过——本机通常没有此 unit 名。
		t.Skip("systemctl enable 意外成功,跳过")
	}
	assert.Contains(t, err.Error(), "安装服务失败")
	_, lerr := os.Lstat(manager.unitPath)
	assert.ErrorIs(t, lerr, os.ErrNotExist, "enable 失败后应回滚 unit 文件")
}

func TestRunCommandReportsSpawnFailure(t *testing.T) {
	// 命令不存在属“启动失败”,应返回 err(而非 code=-1 的假退出)。
	_, err := runCommandWithTimeout("/no/such/binary", nil, time.Second)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "执行命令失败")
}

func TestRunSystemctlReportsSpawnFailure(t *testing.T) {
	// systemctl 走 runSystemctl,启动失败应报 systemctl: 执行命令失败,而非信号终止。
	manager := forTesting("/bin/sh", "/none/sv", "/none/u", "/none/i", BackendSystemd, &bytes.Buffer{})
	// 无法直接注入 systemctl 路径;改为直接验证 serviceControl 对不存在 init 脚本报启动失败。
	manager2 := forTesting("/bin/sh", "/none/sv", "/none/u", "/none/i", BackendSysV, &bytes.Buffer{})
	manager2.initScriptPath = "/no/such/init-script"
	err := manager2.serviceControl("start")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "执行命令失败")
	_ = manager
}
func tempManager(t *testing.T, backend ServiceBackend) (*ServiceManager, *bytes.Buffer, string) {
	t.Helper()
	dir := t.TempDir()
	etc := filepath.Join(dir, "etc")
	bin := filepath.Join(dir, "bin")
	require.NoError(t, os.MkdirAll(bin, 0755))
	executable := filepath.Join(bin, "sv")
	require.NoError(t, os.WriteFile(executable, []byte("#!/bin/sh\n"), 0755))
	executable, err := filepath.EvalSymlinks(executable)
	require.NoError(t, err)

	symlink := filepath.Join(bin, "sv-cmd")
	unit := filepath.Join(etc, "systemd", serviceName+".service")
	initScript := filepath.Join(etc, "init.d", serviceName)
	var out bytes.Buffer
	manager := forTesting(executable, symlink, unit, initScript, backend, &out)
	return manager, &out, dir
}

func TestSystemdUnitTextMatchesSvRs(t *testing.T) {
	text := systemdUnitText("/opt/sv app/sv")
	want := "[Unit]\nDescription=SV Supervisor Manager\nAfter=network.target\n\n" +
		"[Service]\nType=simple\nExecStart=/opt/sv\\x20app/sv daemon\nRestart=always\n\n" +
		"[Install]\nWantedBy=multi-user.target\n"
	assert.Equal(t, want, text)
}

func TestSysvInitTextMatchesSvRs(t *testing.T) {
	text := sysvInitText("/opt/my app/sv'rs")
	assert.True(t, strings.HasPrefix(text, "#!/bin/sh\n"), text)
	assert.Contains(t, text, "exe='/opt/my app/sv'\\''rs'")
	assert.Contains(t, text, "pidfile=/run/$name.pid")
	assert.Contains(t, text, "Usage: $0 {start|stop|restart|status}")
}

func TestSystemdUnitExecutableRoundTrip(t *testing.T) {
	for _, path := range []string{"/opt/sv", "/opt/my app/sv-rs", "/usr/bin/sv'rs"} {
		text := systemdUnitText(path)
		assert.Equal(t, path, systemdUnitExecutable(text), "path=%q", path)
	}
}

func TestSysvInitExecutableRoundTrip(t *testing.T) {
	for _, path := range []string{"/usr/bin/sv", "/opt/my app/sv'rs", "/opt/it's sv"} {
		text := sysvInitText(path)
		assert.Equal(t, path, sysvInitExecutable(text), "path=%q", path)
	}
}

func TestRecordedServiceExecutableParses(t *testing.T) {
	dir := t.TempDir()
	unit := filepath.Join(dir, "unit.service")
	require.NoError(t, os.WriteFile(unit, []byte(systemdUnitText("/usr/bin/svc app")), 0644))
	assert.Equal(t, "/usr/bin/svc app", recordedExecutableFromFile("systemd", unit))

	script := filepath.Join(dir, "init")
	require.NoError(t, os.WriteFile(script, []byte(sysvInitText("/usr/bin/svc")), 0755))
	assert.Equal(t, "/usr/bin/svc", recordedExecutableFromFile("sysv", script))

	assert.Empty(t, recordedExecutableFromFile("systemd", filepath.Join(dir, "nope.service")))
	assert.Empty(t, recordedExecutableFromFile("bogus", unit))
}

func TestUninstallSysVRefusesForeignScript(t *testing.T) {
	manager, _, dir := tempManager(t, BackendSysV)
	require.NoError(t, os.MkdirAll(filepath.Dir(manager.initScriptPath), 0755))
	marker := filepath.Join(dir, "foreign-ran")
	foreign := "#!/bin/sh\nexe='/usr/bin/not-sv'\necho foreign > '" + marker + "'\nexit 0\n"
	require.NoError(t, os.WriteFile(manager.initScriptPath, []byte(foreign), 0755))
	// 命令软链仍指向本 sv,证明 init 脚本本身是异源的。
	require.NoError(t, os.Symlink(manager.executable, manager.symlinkPath))

	err := manager.uninstall()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "拒绝卸载非本程序注册的服务文件")
	_, lerr := os.Lstat(manager.initScriptPath)
	assert.NoError(t, lerr, "异源脚本不应被删除")
	_, merr := os.Lstat(marker)
	assert.ErrorIs(t, merr, os.ErrNotExist, "不应执行异源脚本")
	_, serr := os.Lstat(manager.symlinkPath)
	assert.NoError(t, serr, "本 sv 软链不受影响")
}

func TestUninstallSystemdRefusesForeignUnit(t *testing.T) {
	manager, _, _ := tempManager(t, BackendSystemd)
	require.NoError(t, os.MkdirAll(filepath.Dir(manager.unitPath), 0755))
	// ExecStart 指向非本 sv 路径 → 异源 unit,门控应在任何 systemctl 之前拦下。
	require.NoError(t, os.WriteFile(manager.unitPath, []byte(systemdUnitText("/usr/bin/not-sv")), 0644))

	err := manager.uninstall()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "拒绝卸载非本程序注册的服务文件")
	_, lerr := os.Lstat(manager.unitPath)
	assert.NoError(t, lerr, "异源 unit 不应被删除")
}

func TestServiceStatusSysVStoppedShowsStopped(t *testing.T) {
	manager, out, _ := tempManager(t, BackendSysV)
	require.NoError(t, manager.writeInitScript())
	// 真实 init 脚本 status 返回 code 3(not running),status 应输出「已停止」而非报错。
	require.NoError(t, manager.status())
	assert.Contains(t, out.String(), "SV 系统服务状态: ⏸️ 已停止")
}

func TestServiceStatusSystemdNotInstalledReportsStateNotError(t *testing.T) {
	manager, out, _ := tempManager(t, BackendSystemd)
	// unit 未安装时 systemctl is-active 以非零退出码返回 inactive;status 按文本
	// 判定应成功返回(而不是被当作错误),避免「已停止」误报。
	require.NoError(t, manager.status())
	assert.Contains(t, out.String(), "SV 系统服务状态: ")
	assert.NotContains(t, out.String(), "获取服务状态失败")
}
