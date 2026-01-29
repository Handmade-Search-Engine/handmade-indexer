import requests
from bs4 import BeautifulSoup
from urllib.parse import urlparse
import urllib.robotparser as txtrobots
import nltk
from supabase import create_client, Client
from dotenv import load_dotenv
import os
import time
import ssl

ssl._create_default_https_context = ssl._create_unverified_context

def get_soup(url: str) -> BeautifulSoup:
    response = requests.get(url)
    soup = BeautifulSoup(response.text, 'html.parser')
    return soup

def get_keywords(content: str):
    tokens = nltk.RegexpTokenizer(r'\w+').tokenize(content.lower())
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

previous_hostname = ""
previous_hostname_soup = None

hostdelays = {}

while True:
    queue = supabase.table('known_pages').select("*").execute().data
    if len(queue) == 0:
        break

    url = urlparse(queue[0]['url'])
    print(len(queue), ":", url.geturl())
    hostname = url.hostname

    if hostname not in hostdelays:
        rp = txtrobots.RobotFileParser()
        rp.set_url("https://" + hostname + "/robots.txt")
        rp.read()
        delay = rp.crawl_delay("*")
        if delay:
            hostdelays[hostname] = float(delay)
        else:
            hostdelays[hostname] = 3

    print(f"waiting: {hostdelays[hostname]} seconds")
    time.sleep(hostdelays[hostname])

    soup = get_soup(url.geturl())
    keywords = get_keywords(soup.get_text("\n"))
        
    hostname_soup = previous_hostname_soup
    if hostname != previous_hostname:
        hostname_soup = get_soup('https://'+hostname)

    title = ""
    if soup.title:
        title = soup.title.contents[0]
        if hostname_soup.title:
            if title == hostname_soup.title.contents[0]:
                header = soup.find('h1')
                if header:
                    title = header.contents[0]

    res = (
        supabase.table("sites")
           .upsert({"url": url.geturl(), "doc_length": len(keywords), "title": title}, on_conflict="url")
           .execute()
        )
    site_id = res.data[0]['site_id']

    posting_rows = []
    words = list(keywords.keys())

    existing_keywords = (
        supabase.table('keywords')
        .select("*")
        .in_("keyword", words)
        .execute()
    ).data

    existing_map = {
        row["keyword"]: row
        for row in existing_keywords
    }

    keyword_upserts = []
    for word in words:
        if word in existing_map:
            keyword_upserts.append({
                "keyword": word,
                "document_frequency": existing_map[word]['document_frequency'] + 1
            })
        else:
            keyword_upserts.append({
                "keyword": word,
                "document_frequency": 1
            })


    supabase.table('keywords').upsert(
        keyword_upserts,
        on_conflict="keyword"
    ).execute()

    keyword_rows = (
        supabase.table('keywords')
        .select('keyword_id', 'keyword')
        .in_('keyword', words)
        .limit(len(keywords.keys()))
        .execute()
    ).data

    keyword_id_map = {
        row['keyword']: row['keyword_id']
        for row in keyword_rows
    }

    print(f"found {len(keyword_id_map.keys())} keywords")

    posting_rows = []
    for word, positions in keywords.items():
        posting_rows.append({
            "keyword_id": keyword_id_map[word],
            "site_id": site_id,
            "term_frequency": len(positions),
            "positions": positions
        })

    response = (
            supabase.table("postings")
            .upsert(posting_rows)
            .execute()
        )
    
    supabase.table('known_pages').delete().eq("url", url.geturl()).execute()
    previous_hostname = hostname
    previous_hostname_soup = hostname_soup