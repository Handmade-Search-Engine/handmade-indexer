import requests
from bs4 import BeautifulSoup
from urllib.parse import urlparse, ParseResult
import urllib.robotparser as txtrobots
from urllib3.exceptions import ProtocolError
from requests.exceptions import ConnectionError
import nltk
from nltk.stem import *
from supabase import create_client, Client
from dotenv import load_dotenv
import os
import time
import ssl
from datetime import datetime
from typing import List, Dict, Any

ssl._create_default_https_context = ssl._create_unverified_context

def get_soup(url: str) -> BeautifulSoup:
    try:
        response = requests.get(url)
    except Exception as e:
        print(e)
        raise e
    soup = BeautifulSoup(response.text, 'html.parser')
    return soup

def get_keywords(content: str):
    tokens = nltk.RegexpTokenizer(r'\w+').tokenize(content.lower())
    tagged = nltk.pos_tag(tokens)

    keywords = {}

    irrelevent_tags = ["DT", ":", "IN", "TO", "PRP", "JJ"]

    stemmer = PorterStemmer()
    for i, (word, tag) in enumerate(tagged):
        if tag in irrelevent_tags:
            continue

        stem = stemmer.stem(word)

        if stem in keywords:
            keywords[stem].append(i)
        else:
            keywords[stem] = [i]

    return keywords

def get_page_title(url: ParseResult, page_soup, hostname_soup) -> str:
    if not page_soup.title:
        return ""
    
    if len(page_soup.title.contents) == 0:
        return ""

    title: str = page_soup.title.contents[0]
        
    if not hostname_soup.title:
        return title
    
    if url.path == "/" or url.path == "":
        return title
    
    if title != hostname_soup.title.contents[0]:
        return title
    
    # page and hostname have the same title
    header = page_soup.find('h1')
    if not header:
        return title
        
    return header.contents[0].get_text()



load_dotenv()

SUPABASE_URL: str = os.environ.get("SUPABASE_URL")
key: str = os.environ.get("SUPABASE_KEY")
supabase: Client = create_client(SUPABASE_URL, key)

previous_hostname = ""
previous_hostname_soup = None

hostdelays = {}

while True:
    queue: List[Dict[str, str]] = supabase.table('known_pages').select("*").execute().data # type: ignore
    if len(queue) == 0:
        break

    i = 0
    while True:
        url: ParseResult = urlparse(queue[i]['url'].strip('/'))
        hostname = url.hostname
        if hostname == None:
            quit()
        
        if i > len(queue):
            break

        if hostname in hostdelays:
            delay = hostdelays[hostname]['delay']
            last_crawl = hostdelays[hostname]['last_crawl']
            seconds = (datetime.now() - last_crawl).total_seconds()
            if seconds > delay:
                print(f"{hostname}: {seconds}s > {delay}s")
                break
        else:
            rp = txtrobots.RobotFileParser()
            rp.set_url("https://" + hostname + "/robots.txt")
            rp.read()
            delay = rp.crawl_delay("*")
            if delay:
                hostdelays[hostname] = {"delay": float(delay), "last_crawl": datetime.now()}
                break
            else:
                hostdelays[hostname] = {"delay":3, "last_crawl": datetime.now()}
                break

        i += 1

    print(len(queue), ":", url.geturl())
    hostdelays[hostname]['last_crawl'] = datetime.now()

    try:
        page_soup = get_soup(url.geturl())
    except (ProtocolError, ConnectionError) as e:
        print(f"Could not fetch {url.geturl()} due to {e}")
        supabase.table('known_pages').delete().eq("url", url.geturl()).execute()
        continue

    keywords = get_keywords(page_soup.get_text("\n"))
        
    hostname_soup = previous_hostname_soup
    if hostname != previous_hostname:
        hostname_soup = get_soup('https://'+hostname)

    title = get_page_title(url, page_soup, hostname_soup)

    description = ""
    if url.path == "" or url.path == "/":
        description_tag = page_soup.find('meta', {'name': 'description'})
        if description_tag:
            description = description_tag.get('content')
        else:
            first_paragraph = page_soup.p
            if first_paragraph:
                description = first_paragraph.text
    
    res = (
        supabase.table("sites")
           .upsert({"url": url.geturl(), "doc_length": len(keywords), "title": title, "description": description}, on_conflict="url")
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
    new_keywords = 0
    keyword_upserts = []
    for word in words:
        if word in existing_map:
            keyword_upserts.append({
                "keyword": word,
                "document_frequency": existing_map[word]['document_frequency'] + 1
            })
        else:
            new_keywords += 1
            keyword_upserts.append({
                "keyword": word,
                "document_frequency": 1
            })


    if len(keyword_upserts) == 0:
        print(f"{url.geturl()} has no keywords")
        supabase.table('known_pages').delete().eq("url", url.geturl()).execute()
        continue

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

    print(f"found {new_keywords} new keywords and {len(keyword_id_map.keys()) - new_keywords} existing")

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