#!/usr/bin/env python3
from pathlib import Path
import re

p = Path(__file__).resolve().parent.parent / "openapi.yaml"
text = p.read_text()

def camel_to_snake(s):
    return re.sub(r'([a-z0-9])([A-Z])', r'\1_\2', s).lower()

targets = [
    "pageSize","pageToken","orderBy","updateMask",
    "contentMd","contentHtml","publishedAt",
    "createdAt","updatedAt","viewCount","nextPageToken"
]

for t in sorted(targets, key=len, reverse=True):
    text = re.sub(rf'\b{t}\b', camel_to_snake(t), text)

p.write_text(text)
