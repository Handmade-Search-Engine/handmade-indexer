import requests
from bs4 import BeautifulSoup
from urllib.parse import urlparse
import nltk
from supabase import create_client, Client
from dotenv import load_dotenv
import os
import pickle

QUEUE_PICKLE_PATH = "queue.pkl"

def load_queue(default_queue: list[str]) -> list[str]:
    if os.path.exists(QUEUE_PICKLE_PATH):
        with open(QUEUE_PICKLE_PATH, "rb") as f:
            return pickle.load(f)
    return default_queue.copy()

def save_queue(queue: list[str]) -> None:
    with open(QUEUE_PICKLE_PATH, "wb") as f:
        pickle.dump(queue, f)

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

    keywords = {}

    irrelevent_tags = ["DT", ":", "IN", "TO", "PRP", "JJ"]

    for i, (word, tag) in enumerate(tagged):
        if tag in irrelevent_tags:
            continue

        if word in keywords:
            keywords[word].append(i)
        else:
            keywords[word] = [i]

    return keywords

load_dotenv()

url: str = os.environ.get("SUPABASE_URL")
key: str = os.environ.get("SUPABASE_KEY")
supabase: Client = create_client(url, key)

default_queue = ["https://leylacornellportfolio.ca/", "https://nicolasgatien.com"]
queue = load_queue(default_queue)
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
    keywords = get_keywords(soup.get_text("\n"))

    res = (
        supabase.table("sites")
           .insert({"url": url.geturl(), "doc_length": len(keywords)})
           .execute()
        )
    site_id = res.data[0]['site_id']

    for hyperlink in links:
        link = urlparse(hyperlink)
        if link.hostname not in allowed_hostnames:
            continue
        queue.append(link.geturl())

    posting_rows = []
    for word, positions in keywords.items():
        keyword_response = (
            supabase.table("keywords")
            .select("keyword_id, document_frequency")
            .eq("keyword", word)
            .execute()
                    )
        if len(keyword_response.data) > 0:
            keyword_id = keyword_response.data[0]['keyword_id']
            (
                supabase.table('keywords')
                .update({"document_frequency": keyword_response.data[0]['document_frequency'] + 1})
                .eq("keyword_id", keyword_id)
                .execute()
             )
        else:
            keyword_insert = (
                supabase.table('keywords')
                .insert({
                    "keyword": word,
                    "document_frequency": 1
                })
                .execute()
            )
            keyword_id = keyword_insert.data[0]['keyword_id']

        posting_rows.append({
            "keyword_id": keyword_id,
            "site_id": site_id,
            "term_frequency": len(positions),
            "positions": positions
        })

    #if (soup.title):
    #    title_keywords = get_keywords(soup.title.string)
    #    for word in title_keywords:
    #        keyword_data.append({"keyword": word, "url": url.geturl(), "score": 5})

    response = (
            supabase.table("postings")
            .insert(posting_rows)
            .execute()
        )
    
    queue.pop(0)
    print(queue)
    save_queue(queue)