package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

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

func createHTTPRequest(url string) *http.Request {
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		panic(err)
	}
	request.Header.Set("User-Agent", "Handmade_Web_Crawler")
	return request
}

func getRobots(hostname string) (bool, Robots) {
	httpClient := &http.Client{}

	url := "https://" + hostname + "/robots.txt"
	request := createHTTPRequest(url)

	_, err := net.LookupHost(hostname)
	if err != nil {
		println(hostname + " does not exist")
		return false, Robots{}
	}

	resp, err := httpClient.Do(request)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		println("Error loading page: " + resp.Status)
		// body, _ := io.ReadAll(resp.Body)
		// println(string(body))
		return false, Robots{}
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		panic(err)
	}

	robots := parseRobots(doc.Text())
	return true, robots
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseKey := os.Getenv("SUPABASE_KEY")

	supabaseClient, err := supabase.NewClient(supabaseURL, supabaseKey, &supabase.ClientOptions{})
	if err != nil {
		fmt.Println("Failed to initalize the client: ", err)
	}

	allowedHostnameObjects := []AllowedHostname{}
	_, err = supabaseClient.From("allowed_hostnames").Select("*", "", false).ExecuteTo(&allowedHostnameObjects)
	if err != nil {
		panic(err)
	}

	allowedHostnames := []string{}
	for i := 0; i < len(allowedHostnameObjects); i++ {
		allowedHostnames = append(allowedHostnames, allowedHostnameObjects[i].URL)
	}

	httpClient := &http.Client{}

	for true {
		queue := []Site{}
		_, err = supabaseClient.From("queue").Select("*", "", false).ExecuteTo(&queue)
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

		hasRobots, robots := getRobots(currentURL.Hostname())
		if hasRobots == false {
			robots = Robots{}
			robots.agentRules = make(map[string]UserAgent)
			robots.agentRules["*"] = UserAgent{crawlDelay: 0, contentSignal: map[string]bool{"search": true}}
		}

		time.Sleep(time.Duration(robots.agentRules["*"].crawlDelay * int(time.Second)))

		request := createHTTPRequest(currentURL.String())

		disallowed := false
		for i := 0; i < len(robots.agentRules["*"].disallow); i++ {
			if strings.Contains(currentURL.String(), robots.agentRules["*"].disallow[i]) {
				disallowed = true
				println(robots.agentRules["*"].disallow[i] + " Is contained in " + currentURL.String())
				break
			}
		}
		if disallowed == true {
			_, _, err = supabaseClient.From("queue").Delete("", "").Eq("url", currentURL.String()).Execute()
			continue
		}

		resp, err := httpClient.Do(request)
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			println("Error loading page: " + resp.Status)
			_, _, err = supabaseClient.From("queue").Delete("", "").Eq("url", currentURL.String()).Execute()
			continue
		}

		if strings.Contains(resp.Header.Get("Content-Type"), "text/html") == false {
			println("Attempting to Parse Non-Text Page -- Skipping")
			_, _, err = supabaseClient.From("queue").Delete("", "").Eq("url", currentURL.String()).Execute()
			continue
		}
		doc, err := goquery.NewDocumentFromReader(resp.Body)
		if err != nil {
			panic(err)
		}

		hostname := currentURL.Hostname()

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
			_, err := supabaseClient.From("known_pages").Select("url", "", false).In("url", newLinks).ExecuteTo(&knownPages)
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
				hasRobots, _ := getRobots(hyperlink.Hostname())
				if hasRobots == false {
					continue
				}

				_, _, err = supabaseClient.From("has_robots").Upsert(map[string]interface{}{"hostname": hyperlink.Hostname()}, "hostname", "", "").Execute()
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
			_, _, err = supabaseClient.From("queue").Insert(map[string]interface{}{"url": hyperlink.String()}, false, "", "", "").Execute()
			if err != nil {
				panic(err)
			}
		}

		_, _, err = supabaseClient.From("known_pages").Insert(map[string]interface{}{"url": currentURL.String()}, false, "", "", "").Execute()
		if err != nil {
			panic(err)
		}

		_, _, err = supabaseClient.From("queue").Delete("", "").Eq("url", currentURL.String()).Execute()
		if err != nil {
			panic(err)
		}
	}
}
