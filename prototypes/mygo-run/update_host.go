package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"asterlab-mygo-prototype/internal/updater"
	mygo "github.com/egoist/mygo"
)

type updatePrefs struct {
	AutoCheck         bool `json:"autoCheck"`
	IncludePrerelease bool `json:"includePrerelease"`
}

func defaultUpdatePrefs() updatePrefs {
	verLower := strings.ToLower(updater.Version)
	norm := updater.NormalizeVersion(updater.Version)
	preDefault := strings.Contains(verLower, "rc") ||
		strings.Contains(verLower, "beta") ||
		strings.Contains(verLower, "alpha") ||
		strings.Contains(verLower, "dev") ||
		strings.Contains(verLower, "unsigned") ||
		strings.HasPrefix(norm, "0.")
	return updatePrefs{
		AutoCheck:         true,
		IncludePrerelease: preDefault,
	}
}

func (a *app) updatePrefsPath() string {
	dir := workspaceDataDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "update-prefs.json")
}

func (a *app) loadUpdatePrefs() updatePrefs {
	prefs := defaultUpdatePrefs()
	path := a.updatePrefsPath()
	if path == "" {
		return prefs
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return prefs
	}
	if err := json.Unmarshal(raw, &prefs); err != nil {
		return defaultUpdatePrefs()
	}
	return prefs
}

func (a *app) saveUpdatePrefs() {
	if a.shell == nil {
		return
	}
	path := a.updatePrefsPath()
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	raw, err := json.MarshalIndent(updatePrefs{
		AutoCheck:         a.shell.UpdateAutoCheck,
		IncludePrerelease: a.shell.UpdateIncludePre,
	}, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, raw, 0o644)
}

func (a *app) runUpdateOnUI(fn func()) {
	if a.windowReady {
		mygo.RunOnMain(fn)
		return
	}
	fn()
}

func summarizeReleaseNotes(raw string) string {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	var kept []string
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		kept = append(kept, t)
		if len(kept) >= 5 {
			break
		}
	}
	joined := strings.Join(kept, "\n")
	r := []rune(joined)
	if len(r) > 280 {
		return string(r[:280]) + "…"
	}
	return joined
}

func (a *app) wireUpdaterCallbacks() {
	if a.shell == nil {
		return
	}
	prefs := a.loadUpdatePrefs()
	a.shell.AppVersion = updater.Version
	a.shell.BuildCommit = updater.Commit
	a.shell.UpdateAutoCheck = prefs.AutoCheck
	a.shell.UpdateIncludePre = prefs.IncludePrerelease
	if a.shell.UpdateDownloadURL == "" {
		a.shell.UpdateDownloadURL = "https://github.com/" + updater.Repo + "/releases"
	}
	a.shell.OnCheckUpdate = func() {
		a.triggerCheckUpdate()
	}
	a.shell.OnInstallUpdate = func() {
		a.triggerInstallUpdate()
	}
	a.shell.OnOpenUpdatePage = func() {
		target := "https://github.com/" + updater.Repo + "/releases"
		if a.shell != nil && a.shell.UpdateDownloadURL != "" {
			target = a.shell.UpdateDownloadURL
		}
		_ = updater.OpenURL(target)
	}
	prevChanged := a.shell.OnSettingsChanged
	a.shell.OnSettingsChanged = func() {
		a.saveUpdatePrefs()
		if prevChanged != nil {
			prevChanged()
		}
	}
}

func (a *app) startAutoUpdateCheck() {
	if a.shell == nil || !a.shell.UpdateAutoCheck {
		return
	}
	if strings.TrimSpace(os.Getenv("ASTERLAB_DISABLE_AUTO_UPDATE")) != "" {
		return
	}
	go func() {
		time.Sleep(2500 * time.Millisecond)
		a.runUpdateOnUI(func() {
			if !a.updateBusy {
				a.triggerCheckUpdate()
			}
		})
	}()
}

func (a *app) triggerCheckUpdate() {
	if a.shell == nil || a.updateBusy {
		return
	}
	includePre := a.shell.UpdateIncludePre
	a.updateBusy = true
	a.shell.UpdateStatus = string(updater.StatusChecking)
	a.shell.UpdateErr = ""
	a.shell.Invalidate()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		res, err := updater.CheckForUpdate(ctx, updater.Version, includePre)
		a.runUpdateOnUI(func() {
			a.updateBusy = false
			if a.shell == nil {
				return
			}
			if err != nil {
				a.shell.UpdateStatus = string(updater.StatusError)
				a.shell.UpdateErr = err.Error()
				a.shell.Invalidate()
				return
			}
			if res.Latest == nil {
				a.latestRelease = nil
				a.shell.UpdateStatus = string(updater.StatusUpToDate)
				a.shell.UpdateLatestVersion = ""
				a.shell.UpdateNotes = ""
				a.shell.Invalidate()
				return
			}
			a.latestRelease = res.Latest
			a.shell.UpdateDownloadURL = firstNonEmpty(res.Latest.HTMLURL, "https://github.com/"+updater.Repo+"/releases")
			a.shell.UpdateNotes = summarizeReleaseNotes(res.Latest.Body)
			if res.HasUpdate {
				a.shell.UpdateStatus = string(updater.StatusAvailable)
				a.shell.UpdateLatestVersion = res.Latest.TagName
			} else {
				a.shell.UpdateStatus = string(updater.StatusUpToDate)
				a.shell.UpdateLatestVersion = res.Latest.TagName
			}
			a.shell.UpdateErr = ""
			a.shell.Invalidate()
		})
	}()
}

func (a *app) triggerInstallUpdate() {
	if a.shell == nil || a.updateBusy || a.latestRelease == nil {
		return
	}
	rel := a.latestRelease
	a.updateBusy = true
	a.shell.UpdateStatus = string(updater.StatusDownloading)
	a.shell.UpdateErr = ""
	a.shell.Invalidate()

	exePath, _ := os.Executable()
	dataDir := workspaceDataDir()
	pid := os.Getpid()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		_, err := updater.InstallAndScheduleRestartWithState(ctx, rel, exePath, dataDir, pid)
		a.runUpdateOnUI(func() {
			a.updateBusy = false
			if a.shell == nil {
				return
			}
			if err != nil {
				a.shell.UpdateStatus = string(updater.StatusError)
				a.shell.UpdateErr = err.Error()
				a.shell.Invalidate()
				return
			}
			a.shell.UpdateStatus = string(updater.StatusReadyToRestart)
			a.shell.UpdateErr = ""
			a.shell.Invalidate()
			a.saveUpdatePrefs()
			a.saveSession()
			go func() {
				time.Sleep(250 * time.Millisecond)
				mygo.RunOnMain(func() {
					mygo.App.Quit()
				})
				time.Sleep(1200 * time.Millisecond)
				os.Exit(0)
			}()
		})
	}()
}
