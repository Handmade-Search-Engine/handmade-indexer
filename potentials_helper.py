import requests
from bs4 import BeautifulSoup
from urllib.parse import urlparse, ParseResult
import urllib.robotparser as txtrobots
import nltk
from nltk.stem import *
from supabase import create_client, Client
from dotenv import load_dotenv
import os
import time
import ssl
from typing import List, Dict, Any

ssl._create_default_https_context = ssl._create_unverified_context

load_dotenv()

SUPABASE_URL: str = os.environ.get("SUPABASE_URL")
key: str = os.environ.get("SUPABASE_KEY")
supabase: Client = create_client(SUPABASE_URL, key)

while True:
    queue: List[Dict[str, str]] = supabase.table('has_robots').select("*").execute().data # type: ignore
    if len(queue) == 0:
        break

    hostname = queue[0]['hostname']

    print(len(queue), ":", f"https://{hostname}")
    print(f"https://{hostname}/robots.txt")
    choice = input("Options:\n[d] Ban hostname\n[a] Add to queue\n[s] Skip\n[c] Add to Crawl Only\n")

    if choice == 'a':
        (
        supabase.table("queue")
           .insert({"url": f'https://{hostname}'})
           .execute()
        )
        (
        supabase.table("approved_hostnames")
           .upsert({"url": f'{hostname}'}, on_conflict="url")
           .execute()
        )
    elif choice == "c":
        (
        supabase.table("crawl_only")
           .upsert({"hostname": f'{hostname}'}, on_conflict="hostname")
           .execute()
        )
    elif choice == "s":
        pass
    else:
        (
        supabase.table("banned_hostnames")
           .upsert({"url": f'{hostname}'}, on_conflict="url")
           .execute()
        )
    
    supabase.table('has_robots').delete().eq("hostname", hostname).execute()


