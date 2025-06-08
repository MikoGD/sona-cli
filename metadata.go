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
	Number    uint8     `xml:"number"`
	Length    uint32    `xml:"length"`
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
}

type MetaDisc struct {
	XMLName  xml.Name    `xml:"disc"`
	Releases ReleaseList `xml:"release-list"`
}

type MetaData struct {
	XMLName xml.Name `xml:"metadata"`
	Disc    MetaDisc `xml:"disc"`
}

func GetMetaDataForCD() (*MetaData, error) {
	disc, err := discid.Read("/dev/disk8")
	if err != nil {
		errorMessage := fmt.Sprintf("Failed to read disc ID: %s\n", err)
		return nil, errors.New(errorMessage)
	}

  log.Printf("Disc ID: %s\n", disc.ID())

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
  log.Printf("Disc TOC: %s \n", toc)
	queries.Set("toc", toc)
	URL.RawQuery = queries.Encode()
  log.Printf("Creating request for URL: %s\n", URL.String())
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

  log.Printf("metadata %v\n", metadata)

	log.Printf("Album name: %s\n", metadata.Disc.Releases.Release[0].Title)

	return &metadata, nil
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

func GetFlacTags(metadata *MetaData) map[string]FlacTags {
	songs := make(map[string]FlacTags)
	release := metadata.Disc.Releases.Release[0]
	tracks := release.MediumList.Medium[0].TrackList.Track
	albumName := release.Title
	trackTotal := release.MediumList.Medium[0].TrackList.Count
	discNumber := release.MediumList.Medium[0].Position
	discTotal := len(release.MediumList.Medium[0].DiscList.Disc)

	albumArtist := make([]string, len(release.AristCredit.NameCredit))
	for index, nameCredit := range release.AristCredit.NameCredit {
		albumArtist[index] = nameCredit.Artist.Name
	}

	artistType := release.AristCredit.NameCredit[0].Artist.Type

	for _, track := range tracks {
		tags := FlacTags{}
		tags.Title = track.Recording.Title

		artists := make([]string, len(track.Recording.ArtistCredit.NameCredit))
		for index, nameCredit := range track.Recording.ArtistCredit.NameCredit {
			artists[index] = nameCredit.Artist.Name
		}
		tags.Artist = artists

		tags.Album = albumName
		tags.AlbumArtist = albumArtist
		tags.TrackNumber = track.Number
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

		songs[track.Recording.Title] = tags
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

func AddFLACTags(songs map[string]FlacTags) error {
	for song, tags := range songs {
    fileNameWithoutTags := fmt.Sprintf("%s-no-tags.flac", song)
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

    comments.Add("TITLE", tags.Title)

		for _, artist := range tags.Artist {
			comments.Add("ARTIST", artist)
		}
    
    comments.Add("ALBUM", tags.Album)

    for _, albumArtist := range tags.AlbumArtist {
      comments.Add("ALBUM_ARTIST", albumArtist)
    }

    comments.Add("TRACK_NUMBER", strconv.Itoa(int(tags.TrackNumber)))
    comments.Add("TRACK_TOTAL", strconv.Itoa(int(tags.TrackTotal)))
    comments.Add("DISC_NUMBER", strconv.Itoa(int(tags.DiscNumber)))
    comments.Add("DISC_TOTAL", strconv.Itoa(int(tags.DiscTotal)))
    comments.Add("RELEASE_DATE", tags.ReleaseDate)
    comments.Add("LENGTH", strconv.Itoa(int(tags.Length)))

    for _, genre := range tags.Genre {
      comments.Add("GENRE", genre)
    }

    comments.Add("JOIN_PHRASE", tags.JoinPhrase)
    comments.Add("ARTIST_TYPE", tags.ArtistType)

    commentsMeta := comments.Marshal()

    if commentsIndex > 0 {
      flacFile.Meta[commentsIndex] = &commentsMeta
    } else {
      flacFile.Meta = append(flacFile.Meta, &commentsMeta)
    }

    flacFileWithTagsName := fmt.Sprintf("%d. %s.flac", tags.TrackNumber, tags.Title)
    if err := flacFile.Save(flacFileWithTagsName); err != nil {
      return err
    }
	}

  return nil
}
