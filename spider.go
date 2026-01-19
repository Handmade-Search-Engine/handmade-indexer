package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/joho/godotenv"
	"github.com/supabase-community/supabase-go"
)

type AllowedHostname struct {
	ID        int    `json:"id"`
	URL       string `json:"url"`
	Timestamp string `json:"created_at"`
}

type Site struct {
	ID  int    `json:"id"`
	URL string `json:"url"`
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseKey := os.Getenv("SUPABASE_KEY")

	client, err := supabase.NewClient(supabaseURL, supabaseKey, &supabase.ClientOptions{})
	if err != nil {
		fmt.Println("Failed to initalize the client: ", err)
	}

	allowedHostnameObjects := []AllowedHostname{}
	_, err = client.From("allowed_hostnames").Select("*", "", false).ExecuteTo(&allowedHostnameObjects)
	if err != nil {
		panic(err)
	}

	allowedHostnames := []string{}
	for i := 0; i < len(allowedHostnameObjects); i++ {
		allowedHostnames = append(allowedHostnames, allowedHostnameObjects[i].URL)
	}

	for true {
		queue := []Site{}
		_, err = client.From("queue").Select("*", "", false).ExecuteTo(&queue)
		if err != nil {
			panic(err)
		}

		if len(queue) == 0 {
			break
		}

		currentURL, err := url.Parse(queue[0].URL)
		if err != nil {
			panic(err)
		}

		resp, err := http.Get(currentURL.String())
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()

		println(currentURL.String())
		if resp.StatusCode != 200 {
			println("Error loading page: " + resp.Status)
			_, _, err = client.From("queue").Delete("", "").Eq("url", currentURL.String()).Execute()
			continue
		}

		if resp.Header.Get("Content-Type") != "text/html" {
			println("Attempting to Parse Non-Text Page -- Skipping")
			_, _, err = client.From("queue").Delete("", "").Eq("url", currentURL.String()).Execute()
			continue
		}
		doc, err := goquery.NewDocumentFromReader(resp.Body)
		if err != nil {
			panic(err)
		}

		hostname := currentURL.Hostname()
		println(currentURL.String())

		newLinks := []string{}
		doc.Find("a").Each(func(i int, s *goquery.Selection) {
			hyperlink, exists := s.Attr("href")
			if exists == false {
				return
			} else if strings.HasPrefix(hyperlink, "https://") {
				newLinks = append(newLinks, hyperlink)
			} else if strings.HasPrefix(hyperlink, "/") {
				newLinks = append(newLinks, "https://"+hostname+hyperlink)
			} else {
				newLinks = append(newLinks, "https://"+hostname+"/"+hyperlink)
			}
		})

		stringQueue := []string{}
		for i := 0; i < len(queue); i++ {
			stringQueue = append(stringQueue, queue[i].URL)
		}

		knownPages := []Site{}
		if len(newLinks) > 0 {
			_, err := client.From("known_pages").Select("url", "", false).In("url", newLinks).ExecuteTo(&knownPages)
			if err != nil {
				panic(err)
			}
		}

		knownURLs := []string{}
		for i := 0; i < len(knownPages); i++ {
			knownURLs = append(knownURLs, knownPages[i].URL)
		}

		for i := 0; i < len(newLinks); i++ {
			hyperlink, err := url.Parse(newLinks[i])
			if err != nil {
				panic(err)
			}

			if len(hyperlink.Fragment) > 0 {
				println("Points to Fragment")
				continue
			}

			if strings.HasSuffix(hyperlink.String(), ".xml") == true {
				println("Is an XML file")
				continue
			}

			if slices.Contains(allowedHostnames, hyperlink.Hostname()) == false {
				println("Not in allowed hostnames")
				continue
			}
			if slices.Contains(stringQueue, hyperlink.String()) == true {
				println("Already in queue")
				continue
			}
			if slices.Contains(knownURLs, hyperlink.String()) == true {
				println("Already in found")
				continue
			}
			println(hyperlink.String())
			_, _, err = client.From("queue").Insert(map[string]interface{}{"url": hyperlink.String()}, false, "", "", "").Execute()
			if err != nil {
				panic(err)
			}
		}

		_, _, err = client.From("known_pages").Insert(map[string]interface{}{"url": currentURL.String()}, false, "", "", "").Execute()
		if err != nil {
			panic(err)
		}

		_, _, err = client.From("queue").Delete("", "").Eq("url", currentURL.String()).Execute()
		if err != nil {
			panic(err)
		}
	}
}
