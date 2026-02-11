package main

import (
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"

	"github.com/mikogd/hextech/env"
	"github.com/mikogd/maokai"
)

func changeDirectory(path string) {
	if err := os.Chdir(path); err != nil {
		log.Fatalf("Failed to change directory to %s: %s\n", path, err)
	}
}

func start() uint8 {
	env.LoadEnv("./.env")

	loggerConfig := maokai.LoggerConfig{
		LogDirectoryPath: "/var/log/sona-cli",
		LogName: "sona-cli.log",
	}

	logger, err := maokai.CreateLogger(loggerConfig)

	if err != nil {
		errorMessage := fmt.Sprintf("Failed to create logger: %s\n", err)
		logger.CreateErrorLog(errorMessage)
		log.Fatalln(errorMessage)
	}

	metadata, err := GetMetaDataForCD(logger)
	if err != nil {
		errorMessage := fmt.Sprintf("Failed to get meta data: %v", err)
		logger.CreateErrorLog(errorMessage)
		log.Println(errorMessage)
		return 1
	}

	release := GetRelease(metadata, logger)

	pathToMusicFolder := os.Getenv("PATH_TO_DEST_MUSIC")
	if pathToMusicFolder == "" {
		logger.CreateLog("Failed to get PATH_TO_DEST_MUSIC environment variable")
		log.Fatalf("Failed to get PATH_TO_DEST_MUSIC environment variable")
	}
	logger.CreateLog(fmt.Sprintf("Path to folder %s", pathToMusicFolder))

	artistName := release.AristCredit.NameCredit[0].Artist.Name
	albumName := release.Title
	pathToAlbum := path.Join(pathToMusicFolder, artistName, sanitizeSongName(logger, albumName))
	if _, err := os.Stat(pathToAlbum); os.IsNotExist(err) {
		log.Printf("%s doesn't exist, creating directory\n", pathToAlbum)
		logger.CreateLog(fmt.Sprintf("%s doesn't exist, creating directory", pathToAlbum))
		if err = os.MkdirAll(pathToAlbum, 0777); err != nil {
			errorMessage := fmt.Sprintf("Failed to create directory %s: %s\n", pathToAlbum, err)
			log.Println(errorMessage)
			logger.CreateErrorLog(errorMessage)
			return 1
		}
	} else if err != nil {
		// Some other error while trying to check the folder
		errorMessage := fmt.Sprintf("Error checking directory %s: %s", pathToAlbum, err)
		log.Println(errorMessage)
		logger.CreateErrorLog(errorMessage)
		return 1
	}

	startingWorkingDirectory, err := os.Getwd()
	if err != nil {
		errorMessage := fmt.Sprintf("Failed to get the current working directory: %s", err)
		logger.CreateLog(errorMessage) 
		log.Fatalln(errorMessage)
	}

	if err = os.Chdir(pathToAlbum); err != nil {
		errorMessage := fmt.Sprintf("Failed to do change current working directory to %s", pathToAlbum)
		logger.CreateErrorLog(errorMessage)
		log.Println(errorMessage)
		log.Printf("Aborting...")
		return 1
	}

	defer changeDirectory(startingWorkingDirectory)

	if err := RipCD(pathToAlbum, logger); err != nil {
		errorMessage := fmt.Sprintf("Failed to rip CD: %s", err)
		log.Println(errorMessage)
		logger.CreateErrorLog(errorMessage)
		changeDirectory(startingWorkingDirectory)
		os.Exit(1)
	}

	discNumberArg := ""
	if len(os.Args) == 2 {
		discNumberArg = os.Args[1]
	}

	var discNumber int
	if discNumberArg == "" {
		discNumber = 1
	} else {
		discNumber, err = strconv.Atoi(discNumberArg)
		if err != nil {
			errorMessage := fmt.Sprintf("Failed to convert discNumberArg '%s' to int", discNumberArg)
			logger.CreateErrorLog(errorMessage)
			log.Fatalln(errorMessage)
		}
	}

	message := fmt.Sprintf("discNumber: %d", discNumber)
	log.Println(message)
	logger.CreateLog(message)

	songs := GetFlacTags(metadata, release, uint8(discNumber), logger)

	for _, song := range songs {
		message = fmt.Sprintf("song: %s\ntrack number: %d\ntags: %v\n\n", song.Title, song.TrackNumber,  song)
		log.Println(message)
		logger.CreateLog(message)
	}

	for _, song := range songs {
		trackName := fmt.Sprintf("track%02d.flac", song.TrackNumber)
		oldPath := path.Join(pathToAlbum, trackName)

		newFileName := fmt.Sprintf("%02d. %s-no-tags.flac", song.TrackNumber, sanitizeSongName(logger, song.Title))
		newPath := path.Join(pathToAlbum, newFileName)
		logger.CreateLog(fmt.Sprintf("Renaming %s to %s", oldPath, newPath))
		if err := os.Rename(oldPath, newPath); err != nil {
			errorMessage := fmt.Sprintf("Failed to rename %s to %s: %s\n", oldPath, newPath, err)
			log.Println(errorMessage)
			logger.CreateLog(errorMessage)
			changeDirectory(startingWorkingDirectory)
			return 1
		}
	}

	AddFLACTags(songs, metadata, uint8(discNumber), release, logger)

	logger.CreateLog("Cleaning up folder")
	logger.CreateLog("Deleting all *.cdda.wav files")
	log.Printf("Deleting all *.cdda.wav files")
	matches, err := filepath.Glob("*.cdda.wav")
	for _, match := range matches {
		if err := os.Remove(match); err != nil {
			errorMessage := fmt.Sprintf("Failed to remove %s", match)
			logger.CreateErrorLog(errorMessage)
			log.Println(errorMessage)
		}
	}

	logger.CreateLog("Deleting all *-no-tags.flac files")
	log.Printf("Deleting all *-no-tags.flac files")
	matches, err = filepath.Glob("*-no-tags.flac")
	for _, match := range matches {
		if err := os.Remove(match); err != nil {
			logger.CreateLog(fmt.Sprintf("Failed to remove %s", match))
			log.Printf("Failed to remove %s\n", match)
		}
	}

	return 0
}

func main() {
	code := start()
	os.Exit(int(code))
}
