package main

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-flac/flacvorbis/v2"
	"github.com/go-flac/go-flac/v2"
	"github.com/mikogd/maokai"
	"go.uploadedlobster.com/discid"
)

var (
	API_URL    = "https://musicbrainz.org/ws/2"
	USER_AGENT = "Sona/1.0 (mikaelescolin@gmail.com)"
)

type Artist struct {
	XMLName  xml.Name `xml:"artist"`
	Name     string   `xml:"name"`
	SortName string   `xml:"sort-name"`
	Country  string   `xml:"country"`
	Type     string   `xml:"type,attr"`
}

type NameCredit struct {
	XMLName    xml.Name `xml:"name-credit"`
	Artist     Artist   `xml:"artist"`
	JoinPhrase string   `xml:"joinphrase,attr"`
}

type ArtistCredit struct {
	XMLName    xml.Name     `xml:"artist-credit"`
	NameCredit []NameCredit `xml:"name-credit"`
}

type Genre struct {
	XMLName xml.Name `xml:"genre"`
	ID      string   `xml:"id,attr"`
	Name    string   `xml:"name"`
}

type GenreList struct {
	XMLName xml.Name `xml:"genre-list"`
	Genre   []Genre  `xml:"genre"`
}

type Recording struct {
	XMLName          xml.Name     `xml:"recording"`
	Title            string       `xml:"title"`
	ArtistCredit     ArtistCredit `xml:"artist-credit"`
	FirstReleaseDate string       `xml:"first-release-date"`
	GenreList        GenreList    `xml:"genre-list"`
}

type Track struct {
	XMLName   xml.Name  `xml:"track"`
	Number    string    `xml:"number"`
	Length    uint32    `xml:"length"`
	Title     string    `xml:"title"`
	Recording Recording `xml:"recording"`
}

type TrackList struct {
	XMLName xml.Name `xml:"track-list"`
	Count   uint8    `xml:"count,attr"`
	Track   []Track  `xml:"track"`
}

type Disc struct {
	XMLName xml.Name `xml:"disc"`
	Sectors int      `xml:"sectors"`
	Id      string   `xml:"id,attr"`
}

type DiscList struct {
	XMLName xml.Name `xml:"disc-list"`
	Disc    []Disc   `xml:"disc"`
}

type Medium struct {
	XMLName   xml.Name  `xml:"medium"`
	TrackList TrackList `xml:"track-list"`
	DiscList  DiscList  `xml:"disc-list"`
	Position  uint8     `xml:"position"`
	Format    string    `xml:"format"`
}

type MediumList struct {
	XMLName xml.Name `xml:"medium-list"`
	Count   uint8    `xml:"count,attr"`
	Medium  []Medium `xml:"medium"`
}

type Release struct {
	XMLName     xml.Name     `xml:"release"`
	Title       string       `xml:"title"`
	AristCredit ArtistCredit `xml:"artist-credit"`
	MediumList  MediumList   `xml:"medium-list"`
}

type ReleaseList struct {
	XMLName xml.Name  `xml:"release-list"`
	Release []Release `xml:"release"`
	Count   int       `xml:"count,attr"`
}

type MetaDisc struct {
	XMLName  xml.Name    `xml:"disc"`
	Releases ReleaseList `xml:"release-list"`
}

type MetaData struct {
	XMLName  xml.Name     `xml:"metadata"`
	Disc     *MetaDisc    `xml:"disc"`
	Releases *ReleaseList `xml:"release-list"`
}

func GetMetaDataForCD(logger *maokai.FileLogger) (*MetaData, error) {
	CDDriveName, err := getCDDriveDeviceName(logger)
	if err != nil {
		errorMessage := fmt.Sprintf("Failed to get CD Drive: %s", err)
		return nil, errors.New(errorMessage)
	}

	disc, err := discid.Read(CDDriveName)
	if err != nil {
		errorMessage := fmt.Sprintf("Failed to read disc ID: %s\n", err)
		return nil, errors.New(errorMessage)
	}

	log.Printf("Disc ID: %s\n", disc.ID())
	logger.CreateLogf("Disc ID: %s", disc.ID)

	defer disc.Close()

	URLString := fmt.Sprintf("%s/discid/%s", API_URL, disc.ID())
	URL, err := url.Parse(URLString)
	if err != nil {
		errorMessage := fmt.Sprintf("Failed to parse URL %s: %s\n", URL, err)
		return nil, errors.New(errorMessage)
	}

	queries := URL.Query()
	queries.Set("inc", "artists+recordings")
	toc := strings.ReplaceAll(disc.TOCString(), " ", "+")
	logger.CreateLogf("Disc TOC: %s \n", toc)
	queries.Set("toc", toc)
	URL.RawQuery = queries.Encode()
	logger.CreateLogf("Creating request for URL: %s\n", URL.String())
	req, err := http.NewRequest("GET", URL.String(), nil)
	if err != nil {
		errorMessage := fmt.Sprintf("Error creating request: %s\n", err)
		return nil, errors.New(errorMessage)
	}

	req.Header.Set("User-Agent", USER_AGENT)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		errorMessage := fmt.Sprintf("Error making request: %s", err)
		return nil, errors.New(errorMessage)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		errorMessage := fmt.Sprintf("Error reading body: %s\n", err)
		return nil, errors.New(errorMessage)
	}

	metadata := MetaData{}
	if err := xml.Unmarshal(body, &metadata); err != nil {
		errorMessage := fmt.Sprintf("Error parsing xml: %s\n", err)
		return nil, errors.New(errorMessage)
	}

	return &metadata, nil
}

func GetRelease(metadata *MetaData, logger *maokai.FileLogger) Release {
	logger.CreateLog("Getting the release property")
	var releases []Release

	if metadata.Disc != nil {
		releases = metadata.Disc.Releases.Release
	} else if metadata.Releases != nil {
		releases = metadata.Releases.Release
	} else {
		errorMessage := fmt.Sprintf("Incomplete metadata schema can't find releases: %v\n", metadata)
		logger.CreateLog(errorMessage)
		log.Fatalf(errorMessage)
	}

	for _, release := range releases {
		if release.MediumList.Medium[0].Format == "CD" {
			return release
		}
	}

	errorMessage := "No CD release found"
	logger.CreateLog(errorMessage)
	log.Fatalln(errorMessage)

	return Release{}
}

type FlacTags struct {
	Title       string
	Artist      []string
	Album       string
	AlbumArtist []string
	TrackNumber uint8
	TrackTotal  uint8
	DiscNumber  uint8
	DiscTotal   uint8
	ReleaseDate string
	Length      uint32
	Genre       []string
	JoinPhrase  string
	ArtistType  string
}

func GetFlacTags(metadata *MetaData, release Release, discNumber uint8, logger maokai.Logger) []FlacTags {
	logger.CreateLog("Getting flac tags for songs")
	medium := release.MediumList.Medium[discNumber-1]

	tracks := medium.TrackList.Track
	albumName := release.Title
	trackTotal := medium.TrackList.Count
	discTotal := len(medium.DiscList.Disc)

	songs := make([]FlacTags, trackTotal)

	albumArtist := make([]string, len(release.AristCredit.NameCredit))
	for index, nameCredit := range release.AristCredit.NameCredit {
		albumArtist[index] = nameCredit.Artist.Name
	}

	artistType := release.AristCredit.NameCredit[0].Artist.Type
	for i, track := range tracks {
		tags := FlacTags{}

		if track.Title != "" {
			tags.Title = track.Title
		} else {
			tags.Title = track.Recording.Title
		}

		artists := make([]string, len(track.Recording.ArtistCredit.NameCredit))
		for index, nameCredit := range track.Recording.ArtistCredit.NameCredit {
			artists[index] = nameCredit.Artist.Name
		}
		tags.Artist = artists

		trackNumber, err := strconv.Atoi(track.Number)
		if err != nil {
			errorMessage := fmt.Sprintf("Failed to parse track number: %v\n", track.Number)
			logger.CreateErrorLog(errorMessage)
			log.Fatal(errorMessage)
		}
		tags.TrackNumber = uint8(trackNumber)

		tags.Album = albumName
		tags.AlbumArtist = albumArtist
		tags.TrackTotal = trackTotal
		tags.DiscNumber = discNumber
		tags.DiscTotal = uint8(discTotal)
		tags.ReleaseDate = track.Recording.FirstReleaseDate

		genres := make([]string, len(track.Recording.GenreList.Genre))
		for index, genre := range track.Recording.GenreList.Genre {
			genres[index] = genre.Name
		}

		tags.Length = track.Length

		if joinPhrase := track.Recording.ArtistCredit.NameCredit[0].JoinPhrase; joinPhrase != "" {
			tags.JoinPhrase = joinPhrase
		}

		tags.ArtistType = artistType

		songs[i] = tags
		logger.CreateLogf("Flac tags for %s: %v", track.Title, track)
	}

	return songs
}

func ExtractFLACComment(flacFile *flac.File) (*flacvorbis.MetaDataBlockVorbisComment, int, error) {
	var comments *flacvorbis.MetaDataBlockVorbisComment
	var commentsIndex int
	for index, meta := range flacFile.Meta {
		if meta.Type == flac.VorbisComment {
			var err error
			comments, err = flacvorbis.ParseFromMetaDataBlock(*meta)
			if err != nil {
				return nil, -1, err
			}
			commentsIndex = index
		}
	}

	return comments, commentsIndex, nil
}

func getMediumForDiscNumber(discNumber uint8, release Release, logger maokai.Logger) Medium {
	for _, medium := range release.MediumList.Medium {
		if medium.Format == "CD" && medium.Position == discNumber {
			return medium
		}
	}

	errorMessage := fmt.Sprintf("Failed to find medium for disc number %d\n", discNumber)
	logger.CreateErrorLog(errorMessage)
	log.Fatalf(errorMessage)

	return Medium{}
}

func AddFLACTags(songs []FlacTags, metadata *MetaData, discNumber uint8, release Release, logger maokai.Logger) error {
	for _, song := range songs {
		fileNameWithoutTags := fmt.Sprintf("%02d. %s-no-tags.flac", song.TrackNumber, sanitizeSongName(logger, song.Title))
		flacFile, err := flac.ParseFile(fileNameWithoutTags)
		if err != nil {
			return err
		}

		comments, commentsIndex, err := ExtractFLACComment(flacFile)
		if err != nil {
			return err
		}

		if comments == nil && commentsIndex > 0 {
			comments = flacvorbis.New()
		}

		comments.Add("TITLE", song.Title)

		for _, artist := range song.Artist {
			comments.Add("ARTIST", artist)
		}

		comments.Add("ALBUM", song.Album)

		for _, albumArtist := range song.AlbumArtist {
			comments.Add("ALBUMARTIST", albumArtist)
		}

		comments.Add("TRACKNUMBER", strconv.Itoa(int(song.TrackNumber)))
		comments.Add("TRACKTOTAL", strconv.Itoa(int(song.TrackTotal)))
		comments.Add("DISCNUMBER", strconv.Itoa(int(song.DiscNumber)))
		comments.Add("DISCTOTAL", strconv.Itoa(int(song.DiscTotal)))
		comments.Add("RELEASEDATE", song.ReleaseDate)
		comments.Add("LENGTH", strconv.Itoa(int(song.Length)))

		for _, genre := range song.Genre {
			comments.Add("GENRE", genre)
		}

		comments.Add("JOINPHRASE", song.JoinPhrase)
		comments.Add("ARTISTTYPE", song.ArtistType)

		commentsMeta := comments.Marshal()

		if commentsIndex > 0 {
			flacFile.Meta[commentsIndex] = &commentsMeta
		} else {
			flacFile.Meta = append(flacFile.Meta, &commentsMeta)
		}

		var currentTrackNumber uint8 = 0
		for i := 1; i < int(discNumber); i++ {
			medium := getMediumForDiscNumber(uint8(i+1), release, logger)
			currentTrackNumber += medium.TrackList.Count
		}

		flacFileWithTagsName := fmt.Sprintf("%02d. %s.flac",
			currentTrackNumber+song.TrackNumber, sanitizeSongName(logger, song.Title))

		logger.CreateLog(fmt.Sprintf("Saving tags for %s", flacFileWithTagsName))
		if err := flacFile.Save(flacFileWithTagsName); err != nil {
			return err
		}
	}

	return nil
}
