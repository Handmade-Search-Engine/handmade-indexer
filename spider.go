package main

import (
	"fmt"
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

//type ApprovedHostname struct {
//	ID        int    `json:"id"`
//	URL       string `json:"url"`
//	Timestamp string `json:"created_at"`
//}

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
	if resp == nil {
		return false, Robots{}
	}

	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 206 {
		return false, Robots{}
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		panic(err)
	}

	robots := parseRobots(doc.Text())
	return true, robots
}

func getURLsFromTable(tableName string, client *supabase.Client) []string {
	sites := []Site{}
	_, err := client.From(tableName).Select("*", "", false).ExecuteTo(&sites)
	if err != nil {
		panic(err)
	}

	urls := []string{}
	for i := 0; i < len(sites); i++ {
		urls = append(urls, sites[i].URL)
	}

	return urls
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

	approvedHostnames := getURLsFromTable("approved_hostnames", supabaseClient)
	bannedHostnames := getURLsFromTable("banned_hostnames", supabaseClient)

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

		println("\n")
		println("Processing: " + currentURL.String())
		println("Fetching robots.txt for: " + currentURL.Hostname())
		hasRobots, robots := getRobots(currentURL.Hostname())
		if hasRobots == false {
			println(currentURL.Hostname() + " has no robots.txt file")
			robots = Robots{}
			robots.agentRules = make(map[string]UserAgent)
			robots.agentRules["*"] = UserAgent{crawlDelay: 3, contentSignal: map[string]bool{"search": true}}
		}

		println("Waiting " + strconv.Itoa(robots.agentRules["*"].crawlDelay) + "seconds")
		time.Sleep(time.Duration(robots.agentRules["*"].crawlDelay * int(time.Second)))

		request := createHTTPRequest(currentURL.String())

		disallowed := false
		for i := 0; i < len(robots.agentRules["*"].disallow); i++ {
			if strings.Contains(currentURL.String(), robots.agentRules["*"].disallow[i]) {
				disallowed = true
				println("Skipping " + currentURL.String() + " because it is disallowed in robots.txt")
				break
			}
		}
		if disallowed == true {
			_, _, err = supabaseClient.From("queue").Delete("", "").Eq("url", currentURL.String()).Execute()
			continue
		}

		resp, err := httpClient.Do(request)
		if resp == nil {
			// server did not return anything meaningful (no http) and closed the connection
			println(currentURL.String() + " closed request without a meaningful response -- skipping")
			_, _, err = supabaseClient.From("queue").Delete("", "").Eq("url", currentURL.String()).Execute()
			continue
		}
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			println(currentURL.String() + " returned status code: " + resp.Status + " -- skipping")
			_, _, err = supabaseClient.From("queue").Delete("", "").Eq("url", currentURL.String()).Execute()
			continue
		}

		if strings.Contains(resp.Header.Get("Content-Type"), "text/html") == false {
			println(currentURL.String() + " returned non-text or html -- skipping")
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

		println(currentURL.String() + " has " + strconv.Itoa(len(newLinks)) + " hyperlinks")
		for i := 0; i < len(newLinks); i++ {
			hyperlink, err := url.Parse(newLinks[i])
			if err != nil {
				panic(err)
			}

			if len(hyperlink.Fragment) > 0 {
				println("skip: " + hyperlink.String() + " is a fragment")
				continue
			}

			if strings.HasSuffix(hyperlink.String(), ".xml") == true {
				println("skip: " + hyperlink.String() + " is a XML file")
				continue
			}

			if slices.Contains(bannedHostnames, hyperlink.Hostname()) == true {
				println("skip: " + hyperlink.Hostname() + " is on the banned list")
				continue
			}

			if slices.Contains(approvedHostnames, hyperlink.Hostname()) == false {
				hasRobots, _ := getRobots(hyperlink.Hostname())
				if hasRobots == false {
					println("skip: " + hyperlink.Hostname() + " is not approved")
					continue
				}

				println("skip: " + hyperlink.Hostname() + " has robots")
				_, _, err = supabaseClient.From("has_robots").Upsert(map[string]interface{}{"hostname": hyperlink.Hostname()}, "hostname", "", "").Execute()
				continue
			}

			if slices.Contains(stringQueue, hyperlink.String()) == true {
				println("skip: " + hyperlink.String() + " is already on the queue")
				continue
			}

			if slices.Contains(knownURLs, hyperlink.String()) == true {
				println("skip: " + hyperlink.String() + " has already been explored")
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
