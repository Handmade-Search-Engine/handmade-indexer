import requests
from bs4 import BeautifulSoup
from urllib.parse import urlparse
import nltk
from supabase import create_client, Client
from dotenv import load_dotenv
import os

def get_links(soup: BeautifulSoup, url: str):
    links = []

    for link in soup.find_all('a'):
        href = link.get('href')
        if href == "/":
            continue
        links.append(href)

    for i, link in enumerate(links):
        if "https://" in link:
            continue

        if link == "":
            continue

        origin = urlparse(url).hostname

        if link[0] == "/":
            links[i] = "https://" + origin + link
            continue

        links[i] =  "https://" + origin + '/' + link
    
    return links

def get_soup(url: str) -> BeautifulSoup:
    response = requests.get(url)
    soup = BeautifulSoup(response.text, 'html.parser')
    return soup

def get_keywords(content: str):
    tokens = nltk.WhitespaceTokenizer().tokenize(content.lower())
    tagged = nltk.pos_tag(tokens)
    relevent_tags = []

    irrelevent_tags = ["DT", ":", "IN", "TO", "PRP", "JJ"]

    for tag in tagged:
        if tag[1] in irrelevent_tags:
            continue

        relevent_tags.append(tag[0])

    return relevent_tags

load_dotenv()

url: str = os.environ.get("SUPABASE_URL")
key: str = os.environ.get("SUPABASE_KEY")
supabase: Client = create_client(url, key)

queue = ["https://leylacornellportfolio.ca/", "https://nicolasgatien.com"]
allowed_hostnames = []

for start in queue:
    allowed_hostnames.append(urlparse(start).hostname)

while len(queue) > 0:
    url = urlparse(queue[0])
    hostname = url.hostname

    if hostname not in allowed_hostnames:
        queue.pop(0)
        continue
    
    res = (
        supabase.table("sites")
        .select("url")
        .eq("url", url.geturl())
        .execute()
           )
    
    print(res.data)

    if len(res.data) > 0:
        queue.pop(0)
        continue
    
    print("processing: ", url.geturl())

    soup = get_soup(url.geturl())
    links = get_links(soup, url.geturl())

    res = (
        supabase.table("sites")
           .insert({"url": url.geturl(), "text": soup.get_text("\n")})
           .execute()
        )

    for hyperlink in links:
        link = urlparse(hyperlink)
        if link.hostname not in allowed_hostnames:
            continue
        queue.append(link.geturl())

    keywords = get_keywords(soup.get_text("\n"))
    recency_offset = 0.1 / len(keywords)

    keyword_data = []
    for i, word in enumerate(keywords):
        keyword_data.append({"keyword": word, "url": url.geturl(), "score": 1 - (i * recency_offset)})

    if (soup.title):
        title_keywords = get_keywords(soup.title.string)
        for word in title_keywords:
            keyword_data.append({"keyword": word, "url": url.geturl(), "score": 5})

    response = (
            supabase.table("index")
            .insert(keyword_data)
            .execute()
        )
    
    queue.pop(0)