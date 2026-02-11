package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/mikogd/maokai"
)

func convertToFlac(fileName string, logger maokai.Logger) error {
  fileNameTokens := strings.Split(fileName, ".")
  if len(fileNameTokens) != 3 {
    errorMessage := fmt.Sprintf("Split file name contained less than 3 items: %v\n", fileNameTokens)
		logger.CreateLog(strings.Trim(errorMessage, "\n"))
    return errors.New(errorMessage)
  }

  newFileName := fmt.Sprintf("%s.flac", fileNameTokens[0])
	logger.CreateLog(fmt.Sprintf("Running command ffmpeg -i %s -c:a flac -compression_level 5", newFileName))
  cmd := exec.Command("ffmpeg", "-i", fileName, "-c:a", "flac", "-compression_level", "5", newFileName)
  cmd.Stdout = os.Stdout
  cmd.Stderr = os.Stderr

  log.Printf("Converting %s to %s\n", fileName, newFileName)
	logger.CreateLog(fmt.Sprintf("Converting %s to %s", fileName, newFileName))
  if err := cmd.Run(); err != nil {
    return err
  }
  log.Printf("Converted %s to %s\n", fileName, newFileName)
	logger.CreateLog(fmt.Sprintf("Converted %s to %s", fileName, newFileName))

  return nil
}

func RipCD(destPath string, logger maokai.Logger) error {
	CDROM, err := getCDDriveDeviceName(logger)
	if err != nil {
		return err
	}

	logger.CreateLog(fmt.Sprintf("Running command cdparanoia -d %s -Bw", CDROM))
  cmd := exec.Command("cdparanoia", "-d", CDROM, "-Bw")
  cmd.Stdout = os.Stdout
  cmd.Stderr = os.Stderr

  if err := cmd.Run(); err != nil {
    errorMessage := fmt.Sprintf("Failed to run cdparanoia -Bw in %s: %s\n", destPath, err)
		logger.CreateLog(strings.Trim(errorMessage, "\n"))
    return errors.New(errorMessage)
  }

  currentDirectory, err := os.ReadDir(".")
  if err != nil {
    errorMessage := fmt.Sprintf("Failed to read current directory: %s\n", err)
		logger.CreateLog(strings.Trim(errorMessage, "\n"))
    return errors.New(errorMessage)
  }

	logger.CreateLog("Converting ripped songs to flac")

  for _, entry := range currentDirectory {
    if entry.IsDir() {
      continue
    }

    if !strings.Contains(entry.Name(), ".cdda.wav") {
      continue
    }

    if err := convertToFlac(entry.Name(), logger); err != nil {
      errorMessage := fmt.Sprintf("Failed to convert %s to flac: %s\n", entry.Name(), err)
			logger.CreateLog(strings.Trim(errorMessage, "\n"))
      return errors.New(errorMessage)
    }
  }

  return nil
}
