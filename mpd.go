package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"
)

type PlaylistMetadata struct {
	Playlist     string `json:"playlist"`
	PlaylistType string `json:"playlistType"`
	Metadata     struct {
		AssetID         string   `json:"assetId"`
		BaseUrls        []string `json:"baseUrls"`
		SupportsCaching bool     `json:"supportsCaching"`
		Ucp             string   `json:"ucp"`
		Version         string   `json:"version"`
	} `json:"metadata"`
}

type MPD struct {
	XMLName                   xml.Name `xml:"MPD"`
	Text                      string   `xml:",chardata"`
	Xmlns                     string   `xml:"xmlns,attr"`
	Xsi                       string   `xml:"xsi,attr"`
	Xlink                     string   `xml:"xlink,attr"`
	SchemaLocation            string   `xml:"schemaLocation,attr"`
	Clearkey                  string   `xml:"clearkey,attr"`
	Cenc                      string   `xml:"cenc,attr"`
	Profiles                  string   `xml:"profiles,attr"`
	Type                      string   `xml:"type,attr"`
	MediaPresentationDuration string   `xml:"mediaPresentationDuration,attr"`
	MaxSegmentDuration        string   `xml:"maxSegmentDuration,attr"`
	MinBufferTime             string   `xml:"minBufferTime,attr"`
	BaseURL                   string   `xml:"BaseURL"`
	ProgramInformation        string   `xml:"ProgramInformation"`
	Period                    struct {
		Text          string `xml:",chardata"`
		ID            string `xml:"id,attr"`
		Start         string `xml:"start,attr"`
		AdaptationSet []struct {
			Text               string `xml:",chardata"`
			ID                 string `xml:"id,attr"`
			ContentType        string `xml:"contentType,attr"`
			StartWithSAP       string `xml:"startWithSAP,attr"`
			SegmentAlignment   string `xml:"segmentAlignment,attr"`
			BitstreamSwitching string `xml:"bitstreamSwitching,attr"`
			Representation     []struct {
				Text              string `xml:",chardata"`
				ID                string `xml:"id,attr"`
				AudioSamplingRate string `xml:"audioSamplingRate,attr"`
				Bandwidth         string `xml:"bandwidth,attr"`
				MimeType          string `xml:"mimeType,attr"`
				Codecs            string `xml:"codecs,attr"`
				SegmentTemplate   struct {
					Text           string `xml:",chardata"`
					Duration       string `xml:"duration,attr"`
					Timescale      string `xml:"timescale,attr"`
					Initialization string `xml:"initialization,attr"`
					Media          string `xml:"media,attr"`
					StartNumber    string `xml:"startNumber,attr"`
				} `xml:"SegmentTemplate"`
				AudioChannelConfiguration struct {
					Text        string `xml:",chardata"`
					SchemeIdUri string `xml:"schemeIdUri,attr"`
					Value       string `xml:"value,attr"`
				} `xml:"AudioChannelConfiguration"`
			} `xml:"Representation"`
			ContentProtection []struct {
				Text        string `xml:",chardata"`
				SchemeIdUri string `xml:"schemeIdUri,attr"`
				Value       string `xml:"value,attr"`
				DefaultKID  string `xml:"default_KID,attr"`
				Laurl       struct {
					Text    string `xml:",chardata"`
					LicType string `xml:"Lic_type,attr"`
				} `xml:"Laurl"`
			} `xml:"ContentProtection"`
		} `xml:"AdaptationSet"`
	} `xml:"Period"`
}

func getBaseURL(fullURL string) string {
	parsedURL, err := url.Parse(fullURL)
	if err != nil {
		return ""
	}

	basePath := path.Dir(parsedURL.Path)
	if basePath == "/" {
		basePath = ""
	}

	return fmt.Sprintf("%s://%s%s/", parsedURL.Scheme, parsedURL.Host, basePath)
}

func GetPlaylistMetadataByID(url string) (*MPD, error) {
	res, err := http.Get(url)

	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var MPD_Data MPD

	err = xml.Unmarshal(body, &MPD_Data)
	if err != nil {
		return nil, err
	}

	return &MPD_Data, nil
}

func GetPlaylistDuration(mpddata *MPD) float64 {

	duration, err := time.ParseDuration(strings.ToLower(strings.TrimPrefix(mpddata.MediaPresentationDuration, "PT")))

	if err != nil {
		fmt.Printf("failed to parse time duration: %s\n", err.Error())
		return 0
	}

	return duration.Seconds()
}

func DownloadAndConcatSegment(mastertrack string, masterplaylisturl string, initmp4 string, adaptation string, segmentindex int) error {
	file, err := os.OpenFile(mastertrack, os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}
	defer file.Close()

	var url string

	if initmp4 == "init_0.mp4" {
		url = fmt.Sprintf("%ssegment_0_%d.m4s", masterplaylisturl, segmentindex)
	} else {
		url = fmt.Sprintf("%ssegment_%s_%s_%d.m4s", masterplaylisturl, strings.ReplaceAll(strings.ReplaceAll(initmp4, "init_", ""), fmt.Sprintf("_%s.mp4", adaptation), ""), adaptation, segmentindex)
	}

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status when downloading segment: %s", resp.Status)
	}

	_, err = io.Copy(file, resp.Body)
	return err
}

func HandleDownloadTrack(id string, numberofsegments float64, baseurl string, initmp4 string, adaptation string, key string) error {
	segmentCount := int(numberofsegments)

	resp, err := http.Get(fmt.Sprintf("%s%s", baseurl, initmp4))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status while downloading init track: %s", resp.Status)
	}

	out, err := os.Create(initmp4)
	if err != nil {
		return err
	}

	_, err = io.Copy(out, resp.Body)

	for idx := 0; idx < segmentCount; idx++ {
		err := DownloadAndConcatSegment(initmp4, baseurl, initmp4, adaptation, idx+1)

		if err != nil {
			fmt.Println(err)
			return err
		}
	}

	out.Close()

	DecryptPlaylist(id, initmp4, key)

	return nil
}

func DecryptPlaylist(id string, initmp4 string, key string) {
	cmd := exec.Command("ffmpeg", "-decryption_key", key, "-i", initmp4, "-c", "copy", fmt.Sprintf("%s.mp4", id))

	err := cmd.Run()
	if err != nil {
		fmt.Println("Error running ffmpeg command:", err)
		return
	}

	err = os.Remove(initmp4)
	if err != nil {
		fmt.Println("Error deleting old playlist:", err)
		return
	}
}

func Merge(videofile string, audiofile string) {
	cmd := exec.Command("ffmpeg", "-i", videofile, "-i", audiofile, "-c:v", "copy", "-c:a", "copy", "master_merged.mp4")

	err := cmd.Run()
	if err != nil {
		fmt.Println("Error running ffmpeg command:", err)
		return
	}

	err = os.Remove(videofile)
	if err != nil {
		fmt.Println("Error deleting video file:", err)
		return
	}

	err = os.Remove(audiofile)
	if err != nil {
		fmt.Println("Error deleting audio file:", err)
		return
	}

	err = os.Remove("master.mp4")
	if err != nil {
		fmt.Println("Error deleting master file:", err)
		return
	}
}
