package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Calls udevadm info to get the name of the device
//
// Command exeucted `udevadm info --query=property --name=<dev>`
func getDevInfo(dev string) ([]byte, error) {
	cmd := exec.Command("udevadm", "info", "--query=property", "--name="+dev)
	out, err := cmd.Output()

	if err != nil {
		return []byte{}, err
	}

	return out, nil
}

func checkIsCDROM(dev string) (bool, error) {
	out, err := getDevInfo(dev)
	if err != nil {
		return false, err
	}

  scanner := bufio.NewScanner(bytes.NewReader(out))
	isCDROM := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Trim(line, " \t\r\n") == "ID_CDROM_BD=1" {
      return false, nil
		}

		if strings.Trim(line, " \t\r\n") == "ID_CDROM=1" {
			isCDROM = true
		}

		// if strings.HasPrefix(line, "ID_CDROM=") && strings.HasSuffix(line, "1") {
		// 	isCDROM = true
		// }
	}

	if isCDROM {
		return true, nil
	}

	return false, nil
}

func getCDDriveDeviceName() (string, error) {
	// Find all /dev/sr* devices
	matches, err := filepath.Glob("/dev/sr*")
	if err != nil {
		fmt.Println("Error listing /dev/sr*:", err)
		return "", err
	}

	// Check which are actual CD/DVD drives
	for _, dev := range matches {
		isCDROM, err := checkIsCDROM(dev)

		if err != nil {
			return "", err
		}

		if isCDROM {
			return dev, nil
		}
	}

	return "", errors.New("Failed to find CD Drive")
}

func sanitizeSongName(songName string) string {
		pattern := `[\/\\:*?"<>|&!;#~%^\[\]{}()$=@,` + "`" + `'\t\n\r]`
		re := regexp.MustCompile(pattern)
		// Replace all matched characters with underscore
		return re.ReplaceAllString(songName, "_")
}
