This repo is seperated into two different services.

`spider.go` is a web crawler which fetches the contents of pages based on a queue, and finds other links from those pages. It explores the website and keeps track of all the pages it visits for the indexer.

This service is written in Go because I like Go, and it's generally more efficient than python.

`robots.go` is a robots.txt parser.

`indexer.py` is an indexer which reads the contents of the pages found by the spider, and indexes them based on their keywords.

This service is written in Python because I like python, and NLTK is an amazing library which doesn't have a Go counterpart (as far as I'm aware).
