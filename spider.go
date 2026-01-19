package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
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

type Queue struct {
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

	allowedHostnames := []AllowedHostname{}
	_, err = client.From("allowed_hostnames").Select("*", "", false).ExecuteTo(&allowedHostnames)
	if err != nil {
		panic(err)
	}
	fmt.Println(allowedHostnames)

	queue := []Queue{}
	_, err = client.From("queue").Select("*", "", false).ExecuteTo(&queue)
	if err != nil {
		panic(err)
	}
	fmt.Println(queue)

	currentURL, err := url.Parse(queue[0].URL)
	if err != nil {
		panic(err)
	}

	resp, err := http.Get(currentURL.String())
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
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
	fmt.Println(newLinks)
}
