package supervisor

import (
	"fmt"
	"os"
	"path/filepath"
)

// createSymlink 创建命令软链接。返回 created 表示本次新建了链接,
// 以便失败时回滚只清理本次创建的链接(已存在且指向本程序时不动)。
func (sm *ServiceManager) createSymlink() (created bool, err error) {
	if err := sm.requireExecutable(); err != nil {
		return false, err
	}
	targetPath := sm.symlinkPath
	if targetPath == "" {
		return false, fmt.Errorf("软链接路径不能为空")
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return false, fmt.Errorf("创建软链接目录失败: %w", err)
	}

	info, err := os.Lstat(targetPath)
	if err == nil {
		if info.Mode()&os.ModeSymlink == 0 {
			return false, fmt.Errorf("目标路径已存在且不是软链接: %s", targetPath)
		}
		resolvedTarget, resolveErr := filepath.EvalSymlinks(targetPath)
		if resolveErr == nil {
			resolvedExecutable, executableErr := filepath.EvalSymlinks(sm.executable)
			if executableErr == nil && resolvedTarget == resolvedExecutable {
				return false, nil
			}
		}
		return false, fmt.Errorf("目标软链接已存在且指向其他文件: %s", targetPath)
	}
	if !os.IsNotExist(err) {
		return false, fmt.Errorf("检查软链接目标失败: %w", err)
	}
	if err := os.Symlink(sm.executable, targetPath); err != nil {
		return false, fmt.Errorf("创建软链接失败: %w", err)
	}
	if _, err := fmt.Fprintf(sm.out, "🔗 已创建软链接: %s -> %s\n", targetPath, sm.executable); err != nil {
		return true, err
	}
	return true, nil
}

// removeSymlink 移除命令软链接。owned 为归属判定集合(服务文件记录的可执行
// 路径 + 当前 exe);为空时仅以当前 exe 判定。用 readlink 原始目标文本比较,
// 允许目标已被删除(dangling),这样升级后仍能删除指向旧 exe 的软链。
func (sm *ServiceManager) removeSymlink(owned []string) error {
	info, err := os.Lstat(sm.symlinkPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("检查软链接失败: %w", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("拒绝删除非软链接路径: %s", sm.symlinkPath)
	}
	target, err := os.Readlink(sm.symlinkPath)
	if err != nil {
		return fmt.Errorf("读取软链接目标失败: %w", err)
	}
	cleanTarget := resolveLinkTarget(sm.symlinkPath, target)
	for _, candidate := range owned {
		if cleanTarget == filepath.Clean(candidate) {
			if err := os.Remove(sm.symlinkPath); err != nil {
				return fmt.Errorf("删除软链接失败: %w", err)
			}
			_, err := fmt.Fprintf(sm.out, "✅ 已删除软链接: %s\n", sm.symlinkPath)
			return err
		}
	}
	return fmt.Errorf("拒绝删除指向其他文件的软链接: %s", sm.symlinkPath)
}

// serviceFileOwned 判断服务注册文件是否为 sv 安装的资产(防止 uninstall 误删
// 占用本服务名的异源文件)。归属证据二选一:记录的可执行路径等于当前 sv(含
// 升级后仍用同一路径的情形),或等于命令软链当前指向的路径(覆盖升级换路径/
// 旧 exe 已删除,软链 readlink 原文匹配即可,允许 dangling)。
func (sm *ServiceManager) serviceFileOwned(recorded string) bool {
	if recorded == "" {
		return false
	}
	recorded = filepath.Clean(recorded)
	if err := sm.requireExecutable(); err == nil && sm.executable != "" {
		if filepath.Clean(sm.executable) == recorded {
			return true
		}
	}
	info, err := os.Lstat(sm.symlinkPath)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		return false
	}
	target, err := os.Readlink(sm.symlinkPath)
	if err != nil {
		return false
	}
	return resolveLinkTarget(sm.symlinkPath, target) == recorded
}

// requireExecutable 解析并缓存当前可执行文件的绝对路径。
func (sm *ServiceManager) requireExecutable() error {
	if sm.executable != "" {
		return nil
	}
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取可执行文件路径失败: %w", err)
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return fmt.Errorf("解析可执行文件路径失败: %w", err)
	}
	info, err := os.Stat(executable)
	if err != nil {
		return fmt.Errorf("获取可执行文件状态失败: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("可执行文件路径指向目录: %s", executable)
	}
	sm.executable = executable
	return nil
}

// uninstallSymlinkOwned 返回删除命令软链时的归属候选:优先解析服务注册文件里
// 记录的可执行路径(卸载前读取),再追加当前运行的可执行路径。
func (sm *ServiceManager) uninstallSymlinkOwned() []string {
	owned := make([]string, 0, 2)
	if recorded := sm.recordedServiceExecutable(); recorded != "" {
		owned = append(owned, recorded)
	}
	if err := sm.requireExecutable(); err == nil && sm.executable != "" {
		owned = append(owned, sm.executable)
	}
	return owned
}

// recordedServiceExecutable 从本服务注册文件(systemd unit 或 SysV init 脚本)解析
// 安装时记录的可执行路径。
func (sm *ServiceManager) recordedServiceExecutable() string {
	var file, parser string
	switch sm.backend {
	case BackendSystemd:
		file = sm.unitPath
		parser = "systemd"
	case BackendSysV:
		file = sm.initScriptPath
		parser = "sysv"
	default:
		return ""
	}
	return recordedExecutableFromFile(parser, file)
}

// recordedExecutableFromFile 从指定服务文件解析安装时记录的可执行路径。
// systemd unit 的 ExecStart 首段是转义(空格=\x20)后的路径;SysV init 脚本的
// exe='…' 单引号值。解析失败返回空串。
func recordedExecutableFromFile(parser, file string) string {
	if file == "" {
		return ""
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return ""
	}
	text := string(data)
	switch parser {
	case "systemd":
		return systemdUnitExecutable(text)
	case "sysv":
		return sysvInitExecutable(text)
	default:
		return ""
	}
}

// sysVRunlevelLinks 返回指向指定 init 脚本的 SysV runlevel 链接路径列表。
func sysVRunlevelLinks(initScript string) []string {
	root := filepath.Dir(filepath.Dir(initScript))
	paths := make([]string, 0, len(sysvStartRunlevels)+len(sysvStopRunlevels))
	for _, runlevel := range sysvStartRunlevels {
		paths = append(paths, filepath.Join(root, "rc"+runlevel+".d", "S50"+serviceName))
	}
	for _, runlevel := range sysvStopRunlevels {
		paths = append(paths, filepath.Join(root, "rc"+runlevel+".d", "K02"+serviceName))
	}
	return paths
}

// inspectSysVRunlevelLink 判定单个 runlevel 链接是否属于本服务。
func inspectSysVRunlevelLink(path, initScript string) (bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("检查 SysV 启动链接失败: %w", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return false, fmt.Errorf("拒绝删除非软链接路径: %s", path)
	}
	target, err := os.Readlink(path)
	if err != nil {
		return false, fmt.Errorf("读取 SysV 启动链接失败: %w", err)
	}
	resolved := resolveLinkTarget(path, target)
	if resolved != filepath.Clean(initScript) {
		return false, fmt.Errorf("拒绝删除指向其他文件的 SysV 启动链接: %s", path)
	}
	return true, nil
}

// anySysVRunlevelLinkExists 报告是否仍有指向该 init 脚本的 runlevel 链接。
func anySysVRunlevelLinkExists(initScript string) bool {
	for _, path := range sysVRunlevelLinks(initScript) {
		if _, err := os.Lstat(path); err == nil {
			return true
		}
	}
	return false
}

// createSysVRunlevelLinks 创建指向指定 init 脚本的 SysV runlevel 链接(与 sv-rs
// `create_sysv_runlevel_links` 语义一致):缺失则建;已存在且指向本脚本视为成功;
// 占用为普通文件或指向其他目标时拒绝,避免覆盖。任一失败回滚本次已建链接。
func createSysVRunlevelLinks(initScript string) error {
	links := sysVRunlevelLinks(initScript)
	expected := filepath.Clean(initScript)
	created := make([]string, 0, len(links))
	rollback := func() {
		for _, path := range created {
			_ = os.Remove(path)
		}
	}
	for _, path := range links {
		info, err := os.Lstat(path)
		switch {
		case err == nil:
			if info.Mode()&os.ModeSymlink == 0 {
				rollback()
				return fmt.Errorf("目标路径已存在且不是软链接: %s", path)
			}
			target, readErr := os.Readlink(path)
			if readErr != nil {
				rollback()
				return fmt.Errorf("读取 SysV 启动链接失败: %w", readErr)
			}
			if resolveLinkTarget(path, target) != expected {
				rollback()
				return fmt.Errorf("目标软链接已存在且指向其他文件: %s", path)
			}
		case os.IsNotExist(err):
			if dir := filepath.Dir(path); dir != "" {
				if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
					rollback()
					return fmt.Errorf("创建 SysV 启动链接目录失败: %w", mkErr)
				}
			}
			if symErr := os.Symlink(initScript, path); symErr != nil {
				rollback()
				return fmt.Errorf("创建 SysV 启动链接失败: %w", symErr)
			}
			created = append(created, path)
		default:
			rollback()
			return fmt.Errorf("检查 SysV 启动链接失败: %w", err)
		}
	}
	return nil
}

// validateSysVRunlevelLinks 校验预期 runlevel 链接全部存在且属于本服务。
func validateSysVRunlevelLinks(initScript string) error {
	for _, path := range sysVRunlevelLinks(initScript) {
		exists, err := inspectSysVRunlevelLink(path, initScript)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("SysV 启动链接缺失: %s", path)
		}
	}
	return nil
}

// removeSysVRunlevelLinks 清理指向指定 init 脚本的 runlevel 链接;缺失即忽略。
func removeSysVRunlevelLinks(initScript string) error {
	removable := make([]string, 0, len(sysvStartRunlevels)+len(sysvStopRunlevels))
	for _, path := range sysVRunlevelLinks(initScript) {
		exists, err := inspectSysVRunlevelLink(path, initScript)
		if err != nil {
			return err
		}
		if exists {
			removable = append(removable, path)
		}
	}
	for _, path := range removable {
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("删除 SysV 启动链接失败: %w", err)
		}
	}
	return nil
}

// resolveLinkTarget 把 readlink 的原始目标解析成绝对化、clean 的路径。
func resolveLinkTarget(linkPath, target string) string {
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(linkPath), target)
	}
	return filepath.Clean(target)
}
