package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	dropboxAccessToken = ""
	photoLimit         = 3000
)

var (
	// flickr returns json inside of a some wrapper, remove it
	replacer = strings.NewReplacer("jsonFlickrFeed(", "", "})", "}")
)

// Flickr public response object
type Flickr struct {
	Title       string    `json:"title"`
	Link        string    `json:"link"`
	Description string    `json:"description"`
	Modified    time.Time `json:"modified"`
	Generator   string    `json:"generator"`
	Items       []struct {
		Title string `json:"title"`
		Link  string `json:"link"`
		Media struct {
			M string `json:"m"`
		} `json:"media"`
		DateTaken   string    `json:"date_taken"`
		Description string    `json:"description"`
		Published   time.Time `json:"published"`
		Author      string    `json:"author"`
		AuthorID    string    `json:"author_id"`
		Tags        string    `json:"tags"`
	} `json:"items"`
}

func downloadFromFlicker(link string, wg *sync.WaitGroup) (buf bytes.Buffer) {
	defer wg.Done()
	log.Println("Downloading", link)
	client := &http.Client{Timeout: 5 * time.Second}
	res, err := client.Get(link)
	if err != nil {
		return
	}
	io.Copy(&buf, res.Body)
	return
}

func uploadToDropbox(buf bytes.Buffer, link string, wg *sync.WaitGroup) (err error) {
	defer wg.Done()
	log.Println("Uploading", buf.Len(), "to dropbox")
	client := &http.Client{Timeout: 5 * time.Second}

	parts := strings.Split(link, "/")
	file := parts[len(parts)-1]

	uri := fmt.Sprintf("https://content.dropboxapi.com/1/files_put/auto/%s?overwrite=true", file)

	req, err := http.NewRequest("POST", uri, bytes.NewReader(buf.Bytes()))
	if err != nil {
		return
	}

	l := strconv.Itoa(buf.Len())
	req.Header.Set("Authorization", "Bearer "+dropboxAccessToken)
	req.Header.Set("Content-Length", l)
	_, err = client.Do(req)
	return nil
}

func main() {
	client := &http.Client{Timeout: 5 * time.Second}
	linkSet := make(map[string]struct{})
	ch := make(chan string, 10)

	var wg sync.WaitGroup
	go func() {
		for link := range ch {
			wg.Add(2)
			go func(link string) {
				buf := downloadFromFlicker(link, &wg)
				err := uploadToDropbox(buf, link, &wg)
				if err != nil {
					log.Println("Something bad happed", err)
				}
			}(link)
		}
	}()

	var counter int
	for counter <= photoLimit {
		res, err := client.Get("https://api.flickr.com/services/feeds/photos_public.gne?format=json")
		if err != nil {
			log.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := ioutil.ReadAll(res.Body)
		var data Flickr
		json.Unmarshal([]byte(replacer.Replace(string(b))), &data)
		for _, item := range data.Items {
			if _, ok := linkSet[item.Media.M]; ok {
				continue
			}
			linkSet[item.Media.M] = struct{}{}
			ch <- item.Media.M
			counter++
			fmt.Printf("Number of links hit: %d\n", counter)
		}
	}
	close(ch)
	wg.Wait()
}
