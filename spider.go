package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

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

	currentURL := queue[0]
	resp, err := http.Get(currentURL.URL)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		fmt.Println(s.Attr("href"))
	})
	fmt.Println(doc.Children())
}
