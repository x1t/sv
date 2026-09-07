package supervisor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sysVTestPaths(root string) (string, []string) {
	etc := filepath.Join(root, "etc")
	initScript := filepath.Join(etc, "init.d", serviceName)
	paths := make([]string, 0, len(sysvStartRunlevels)+len(sysvStopRunlevels))
	for _, runlevel := range sysvStartRunlevels {
		paths = append(paths, filepath.Join(etc, "rc"+runlevel+".d", "S50"+serviceName))
	}
	for _, runlevel := range sysvStopRunlevels {
		paths = append(paths, filepath.Join(etc, "rc"+runlevel+".d", "K02"+serviceName))
	}
	return initScript, paths
}

func createSysVTestLinks(t *testing.T, initScript string, paths []string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(initScript), 0755))
	require.NoError(t, os.WriteFile(initScript, []byte("#!/bin/sh\n"), 0755))
	for _, path := range paths {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
		require.NoError(t, os.Symlink(initScript, path))
	}
}

func assertMissing(t *testing.T, paths []string) {
	t.Helper()
	for _, path := range paths {
		_, err := os.Lstat(path)
		assert.ErrorIs(t, err, os.ErrNotExist, path)
	}
}

func TestRemoveSysVRunlevelLinksRemovesOwnedLinks(t *testing.T) {
	root := t.TempDir()
	initScript, paths := sysVTestPaths(root)
	createSysVTestLinks(t, initScript, paths)

	require.NoError(t, removeSysVRunlevelLinks(initScript))
	assertMissing(t, paths)
	_, err := os.Stat(initScript)
	require.NoError(t, err)
}

func TestRemoveSysVRunlevelLinksSupportsRelativeLinks(t *testing.T) {
	root := t.TempDir()
	initScript, paths := sysVTestPaths(root)
	require.NoError(t, os.MkdirAll(filepath.Dir(initScript), 0755))
	require.NoError(t, os.WriteFile(initScript, []byte("#!/bin/sh\n"), 0755))
	for _, path := range paths {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
		relative, err := filepath.Rel(filepath.Dir(path), initScript)
		require.NoError(t, err)
		require.NoError(t, os.Symlink(relative, path))
	}

	require.NoError(t, removeSysVRunlevelLinks(initScript))
	assertMissing(t, paths)
}

func TestRemoveSysVRunlevelLinksSupportsDanglingLinks(t *testing.T) {
	root := t.TempDir()
	initScript, paths := sysVTestPaths(root)
	createSysVTestLinks(t, initScript, paths)
	require.NoError(t, os.Remove(initScript))

	require.NoError(t, removeSysVRunlevelLinks(initScript))
	assertMissing(t, paths)
}

func TestRemoveSysVRunlevelLinksRefusesForeignLinkWithoutPartialRemoval(t *testing.T) {
	root := t.TempDir()
	initScript, paths := sysVTestPaths(root)
	createSysVTestLinks(t, initScript, paths)
	other := filepath.Join(root, "other-service")
	require.NoError(t, os.WriteFile(other, []byte("other"), 0644))
	foreign := paths[0]
	require.NoError(t, os.Remove(foreign))
	require.NoError(t, os.Symlink(other, foreign))

	error := removeSysVRunlevelLinks(initScript)
	require.Error(t, error)
	assert.Contains(t, error.Error(), "指向其他文件")
	for _, path := range paths {
		_, statError := os.Lstat(path)
		assert.NoError(t, statError, path)
	}
}

func TestRemoveSysVRunlevelLinksRefusesRegularFileWithoutPartialRemoval(t *testing.T) {
	root := t.TempDir()
	initScript, paths := sysVTestPaths(root)
	createSysVTestLinks(t, initScript, paths)
	regular := paths[0]
	require.NoError(t, os.Remove(regular))
	require.NoError(t, os.WriteFile(regular, []byte("not a link"), 0644))

	error := removeSysVRunlevelLinks(initScript)
	require.Error(t, error)
	assert.Contains(t, error.Error(), "非软链接")
	for _, path := range paths {
		_, statError := os.Lstat(path)
		assert.NoError(t, statError, path)
	}
}

func TestRemoveSysVRunlevelLinksIgnoresMissingLinks(t *testing.T) {
	root := t.TempDir()
	initScript, paths := sysVTestPaths(root)
	require.NoError(t, os.MkdirAll(filepath.Dir(initScript), 0755))
	require.NoError(t, os.WriteFile(initScript, []byte("#!/bin/sh\n"), 0755))

	require.NoError(t, removeSysVRunlevelLinks(initScript))
	assertMissing(t, paths)
}

func TestCreateSysVRunlevelLinksCreatesAndValidates(t *testing.T) {
	root := t.TempDir()
	initScript, _ := sysVTestPaths(root)
	require.NoError(t, os.MkdirAll(filepath.Dir(initScript), 0755))
	require.NoError(t, os.WriteFile(initScript, []byte("#!/bin/sh\n"), 0755))

	require.NoError(t, createSysVRunlevelLinks(initScript))
	require.NoError(t, validateSysVRunlevelLinks(initScript))
	// 幂等:已存在且指向本脚本时成功。
	require.NoError(t, createSysVRunlevelLinks(initScript))
}

func TestValidateSysVRunlevelLinksRequiresAllLinks(t *testing.T) {
	root := t.TempDir()
	initScript, paths := sysVTestPaths(root)
	createSysVTestLinks(t, initScript, paths)

	require.NoError(t, validateSysVRunlevelLinks(initScript))
	require.NoError(t, os.Remove(paths[0]))
	error := validateSysVRunlevelLinks(initScript)
	require.Error(t, error)
	assert.Contains(t, error.Error(), "启动链接缺失")
}

func TestValidateSysVRunlevelLinksRejectsForeignLink(t *testing.T) {
	root := t.TempDir()
	initScript, paths := sysVTestPaths(root)
	createSysVTestLinks(t, initScript, paths)
	other := filepath.Join(root, "other-service")
	require.NoError(t, os.WriteFile(other, []byte("other"), 0644))
	require.NoError(t, os.Remove(paths[0]))
	require.NoError(t, os.Symlink(other, paths[0]))

	error := validateSysVRunlevelLinks(initScript)
	require.Error(t, error)
	assert.Contains(t, error.Error(), "指向其他文件")
}

func TestServiceFileOwnedGate(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	require.NoError(t, os.MkdirAll(bin, 0755))
	executable := filepath.Join(bin, "sv-real")
	require.NoError(t, os.WriteFile(executable, []byte("#!/bin/sh\n"), 0755))
	executable, err := filepath.EvalSymlinks(executable)
	require.NoError(t, err)
	symlink := filepath.Join(bin, "sv")
	manager := &ServiceManager{executable: executable, symlinkPath: symlink}

	// 记录路径 == 当前 exe → 归属。
	assert.True(t, manager.serviceFileOwned(executable))
	// 记录为空 → 不归属。
	assert.False(t, manager.serviceFileOwned(""))
	// 异源且无软链佐证 → 不归属。
	assert.False(t, manager.serviceFileOwned("/usr/bin/not-sv"))

	// 软链指向记录路径(记录 != 当前,旧 exe 不存在即 dangling)→ 归属。
	recorded := filepath.Join(dir, "old", "sv")
	require.NoError(t, os.Symlink(recorded, symlink))
	assert.True(t, manager.serviceFileOwned(recorded))
	assert.False(t, manager.serviceFileOwned("/usr/bin/other"))

	// 软链被普通文件占用时,不再作佐证,仅当前 exe 匹配成立。
	require.NoError(t, os.Remove(symlink))
	require.NoError(t, os.WriteFile(symlink, []byte("occupied"), 0644))
	assert.True(t, manager.serviceFileOwned(executable))
	assert.False(t, manager.serviceFileOwned(recorded))
}
