package main

// Import emoji data from Emojipedia.org
// Useful for rebuilding the emoji data found in the `data/emoji.json` file

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"github.com/teacat/emojiutils"
	"github.com/teacat/emojiutils/utils"
)

type lookup struct {
	Name string
	URL  string
}

func main() {
	fmt.Println("Updating Emoji Definition using Emojipedia…")

	// Grab the latest Apple Emoji Definitions
	res, err := http.Get("https://emojipedia.org/apple/")
	if err != nil {
		panic(err)
	}

	defer res.Body.Close()

	// Load the Apple Emoji HTML page into goquery so that we
	// can query the DOM
	doc, docErr := goquery.NewDocumentFromReader(res.Body)
	if docErr != nil {
		panic(docErr)
	}

	// Create a channel for lookups so that we can do this async
	lookups := []lookup{}

	// Find all emojis on the page
	doc.Find("ul.emoji-grid li").Each(func(i int, s *goquery.Selection) {
		// For each item found, get the band and title
		emojiPage, _ := s.Find("a").Attr("href")
		title, _ := s.Find("img").Attr("title")

		fmt.Printf("Preparing Emoji %d to lookups: %s - %s\n", i, title, emojiPage)

		// Add this specific emoji to the lookups to complete
		lookups = append(lookups, lookup{
			Name: title,
			URL:  "https://emojipedia.org" + emojiPage,
		})
	})

	// Create a WaitGroup so we can fetch multiple emojis and wait for them all to be fetched
	wg := new(sync.WaitGroup)

	// Set the WaitGroup counter as total emojis amount we will fetch
	wg.Add(len(lookups))

	// The maximum parallel crawler to run at the same time
	parallel := make(chan int, 10)

	var emojis []emojiutils.Emoji

	for i, v := range lookups {
		parallel <- 1

		go func(i int, v lookup) {
			fmt.Printf("[%d/%d] Looking up %s\n", i, len(lookups), v.Name)

			// Grab the emojipedia page for this emoji
			res, err := http.Get(v.URL)
			if err != nil {
				fmt.Println(err)
				return
			}

			// Create a new goquery reader
			doc, docErr := goquery.NewDocumentFromReader(res.Body)
			if docErr != nil {
				panic(docErr)
			}

			// Grab the emoji from the "Copy emoji" input field on the HTML page
			emojiString, _ := doc.Find(".copy-paste input[type=text]").Attr("value")

			// Convert the raw Emoji value to our hex key
			hexString := utils.StringToHexKey(emojiString)

			// Add this emoji to our map
			emojis = append(emojis, emojiutils.Emoji{
				Key:        hexString,
				Value:      emojiString,
				Descriptor: v.Name,
			})

			// Print our progress to the console
			fmt.Printf("→ %s, %s, %s\n", hexString, emojiString, v.Name)

			// Mark as finished
			wg.Done()

			// Consume a number so we can start another web crawling
			<-parallel
		}(i, v)
	}

	// Wait for the all emojis to be fetched
	wg.Wait()

	emojiMap := make(map[string]emojiutils.Emoji)
	for _, v := range emojis {
		emojiMap[v.Key] = v
	}

	// Marshal the emojis map as JSON and write to the data directory
	s, _ := json.MarshalIndent(emojiMap, "", "\t")
	os.Mkdir("data", 0644)
	os.WriteFile("data/emoji.json", []byte(s), 0644)
}
