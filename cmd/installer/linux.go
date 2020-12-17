// +build linux

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/buck54321/eco"
)

var (
	svcDir    = filepath.Join(osUser.HomeDir, ".local", "share", "systemd", "user")
	sysApps   = filepath.Join(osUser.HomeDir, ".local", "share", "applications")
	iconDir   = filepath.Join(osUser.HomeDir, ".local", "share", "icons")
	staticDir = filepath.Join(eco.EcoDir, "static")
)

func moveFiles() error {
	// versionB, err := ioutil.ReadFile(filepath.Join("unpacked", "version"))
	// if err != nil {
	// 	return fmt.Errorf("Error parsing version file: %v", err)
	// }
	// version := strings.TrimSpace(string(versionB))

	// Stop any existing running service, ignoring errors since the service
	// might not be running.
	exec.Command("systemctl", "--user", "stop", "eco").Run()

	for _, dir := range []string{svcDir, sysApps, iconDir, staticDir} {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return fmt.Errorf("MkdirAll error for %q: %v", dir, err)
		}
	}

	moves := [][2]string{
		{filepath.Join("unpacked", "eco.service"), svcDir},
		{filepath.Join("unpacked", "ecogui.desktop"), sysApps},
		{filepath.Join("unpacked", "ecogui.png"), iconDir},
		{filepath.Join("unpacked", "ecoservice"), eco.EcoDir},
		{filepath.Join("unpacked", "ecogui"), eco.EcoDir},
		{filepath.Join("unpacked", "version"), eco.EcoDir},
	}

	staticSrc := filepath.Join("unpacked", "static")
	err := filepath.Walk(staticSrc, func(src string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if src == staticSrc {
			return nil
		}
		if fi.IsDir() {
			return fmt.Errorf("unexpected static subdirectory %q", src)
		}
		moves = append(moves, [2]string{src, staticDir})
		return nil
	})
	if err != nil {
		return err
	}

	for _, d := range moves {
		src, destDir := d[0], d[1]
		err := moveFile(src, destDir)
		if err != nil {
			return fmt.Errorf("Error moving file: %v", err)
		}
	}

	err = replaceTokens(filepath.Join(svcDir, "eco.service"), [][2]string{
		{"<ExecStart>", filepath.Join(eco.EcoDir, "ecoservice")},
	})
	if err != nil {
		return fmt.Errorf("Error updating service file: %v", err)
	}

	desktopFile := filepath.Join(sysApps, "ecogui.desktop")
	err = replaceTokens(desktopFile, [][2]string{
		{"<Exec>", filepath.Join(eco.EcoDir, "ecogui")},
		{"<Icon>", filepath.Join(iconDir, "ecogui.png")},
	})
	if err != nil {
		return fmt.Errorf("Error updating service file: %v", err)
	}

	return nil
}

func initService() error {
	// Update the menu entries. Not certain that all linux versions ship with
	// update-desktop-database, or that it is always necessary, so log error
	// but don't quit.
	o, err := exec.Command("update-desktop-database", sysApps).CombinedOutput()
	if err != nil {
		fmt.Printf("ignoring update-desktop-database error: %s: %v \n", string(o), err)
	}

	b, err := exec.Command("systemctl", "--user", "enable", "eco").CombinedOutput()
	if err != nil {
		return fmt.Errorf("Error enabling Eco system service: %s : %v", string(b), err)
	}

	b, err = exec.Command("systemctl", "--user", "start", "eco").CombinedOutput()
	if err != nil {
		return fmt.Errorf("Error starting Eco system service: %s : %v", string(b), err)
	}

	return nil
}

func replaceTokens(path string, subs [][2]string) error {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return fmt.Errorf("ReadFile error: %v", err)
	}
	for _, sub := range subs {
		b = bytes.Replace(b, []byte(sub[0]), []byte(sub[1]), -1)
	}
	return ioutil.WriteFile(path, b, 0644)
}
