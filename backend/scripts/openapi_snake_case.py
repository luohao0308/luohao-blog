#!/usr/bin/env python3
from pathlib import Path
import re

p = Path(__file__).resolve().parent.parent / "openapi.yaml"
text = p.read_text()

def camel_to_snake(s):
    return re.sub(r'([a-z0-9])([A-Z])', r'\1_\2', s).lower()

targets = [
    "pageSize","pageToken","orderBy","updateMask",
    "actorId", "categoryId", "expectedVersion", "articleSlug","sectionTitle","topK","canRetry","checkedAt","semanticReady",
    "contentMd","contentHtml","publishedAt",
    "createdAt","updatedAt","viewCount","nextPageToken","searchOutcome"
]

for t in sorted(targets, key=len, reverse=True):
    text = re.sub(rf'\b{t}\b', camel_to_snake(t), text)

# Runtime validation deliberately returns the domain version-required reason.
# Keep machine query documentation equally strict without generic validation.
text = re.sub(r"(- name: expected_version\n\s+in: query\n)(?!\s+required:)", r"\1                  required: true\n", text)
text = text.replace("        blog.v1.RestoreArticleRevisionRequest:\n            type: object", "        blog.v1.RestoreArticleRevisionRequest:\n            required:\n                - expected_version\n            type: object")
p.write_text(text)
