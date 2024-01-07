package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
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

func isDirExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

const base62 = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func EncodeToBase62(s string) string {
	n := big.NewInt(0).SetBytes([]byte(s))
	base := big.NewInt(62)
	zero := big.NewInt(0)
	mod := &big.Int{}

	var result string
	for n.Cmp(zero) != 0 {
		n.DivMod(n, base, mod)
		result = string(base62[mod.Int64()]) + result
	}
	return result
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

func HandleDownloadTrack(mediatype string, id string, numberofsegments float64, baseurl string, initmp4 string, adaptation string, key string) error {
	segmentCount := int(numberofsegments)

	resp, err := http.Get(fmt.Sprintf("%s%s", baseurl, initmp4))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status while downloading init track: %s", resp.Status)
	}

	if !isDirExists("downloads") {
		err = os.Mkdir("downloads", 0755)
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println("Directory created successfully:", "downloads")
		}
	}

	mastertrack, err := os.Create(fmt.Sprintf("./downloads/%s", initmp4))
	if err != nil {
		return err
	}

	defer mastertrack.Close()

	_, err = io.Copy(mastertrack, resp.Body)

	var wg sync.WaitGroup
	numfilesdownloaded := 0
	files := make([]string, 0)

	fdChannel := make(chan int)
	defer close(fdChannel)

	go func() {
		for iidx := range fdChannel {
			numfilesdownloaded += iidx
			if initmp4 == "init_0.mp4" {
				files = append(files, fmt.Sprintf("segment_0_%d.m4s", numfilesdownloaded))
			} else {
				files = append(files, fmt.Sprintf("segment_en_US_%s_%d.m4s", adaptation, numfilesdownloaded))
			}
		}
	}()

	for idx := 0; idx < segmentCount; idx++ {

		go func(index int) {
			defer wg.Done()

			var url string
			var filename string

			if initmp4 == "init_0.mp4" {
				url = fmt.Sprintf("%ssegment_0_%d.m4s", baseurl, index+1)
				filename = fmt.Sprintf("segment_0_%d.m4s", index+1)
			} else {
				url = fmt.Sprintf("%ssegment_%s_%s_%d.m4s", baseurl, strings.ReplaceAll(strings.ReplaceAll(initmp4, "init_", ""), fmt.Sprintf("_%s.mp4", adaptation), ""), adaptation, index+1)
				filename = fmt.Sprintf("segment_%s_%s_%d.m4s", strings.ReplaceAll(strings.ReplaceAll(initmp4, "init_", ""), fmt.Sprintf("_%s.mp4", adaptation), ""), adaptation, index+1)
			}

			resp, err := http.Get(url)
			if err != nil {
				panic(err)
			}
			defer resp.Body.Close()

			if resp.StatusCode == 404 && idx == int(numberofsegments) {
				numberofsegments -= 1
				fmt.Println("Segment unfortunately wasn't found! No worries it's not your fault! It's my fault for calculating the amount of segments incorrectly! Time to decrypt! ")
				return
			}

			downloadedfile, err := os.Create(fmt.Sprintf("./downloads/%s", filename))
			if err != nil {
				panic(err)
			}

			_, err = io.Copy(downloadedfile, resp.Body)
			if err != nil {
				panic(err)
			}
			defer downloadedfile.Close()

			fdChannel <- 1

		}(idx)
		wg.Add(1)
	}

	wg.Wait()

	for _, segmentName := range files {
		file, err := os.Open(fmt.Sprintf("./downloads/%s", segmentName))
		if err != nil {
			fmt.Println("Error opening segment:", err)
			continue
		}

		_, err = io.Copy(mastertrack, file)
		if err != nil {
			fmt.Println("Error writing segment to master track:", err)
		}
		file.Close()

		err = os.Remove(fmt.Sprintf("./downloads/%s", segmentName))
		if err != nil {
			fmt.Println("Error deleting segment:", err)
		}
	}

	DecryptPlaylist(id, initmp4, key)

	return nil
}

func DecryptPlaylist(id string, initmp4 string, key string) {
	cmd := exec.Command("ffmpeg", "-decryption_key", key, "-i", fmt.Sprintf("./downloads/%s", initmp4), "-c", "copy", fmt.Sprintf("%s.mp4", id))

	err := cmd.Run()
	if err != nil {
		fmt.Println("Error running ffmpeg command:", err)
		return
	}
}

func Merge(videofile string, audiofile string, kid string) {
	cmd := exec.Command("ffmpeg", "-i", videofile, "-i", audiofile, "-c:v", "copy", "-c:a", "copy", fmt.Sprintf("%s_master.mp4", kid))

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
}
