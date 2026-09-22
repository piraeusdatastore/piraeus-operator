#!/usr/bin/env python3
"""Check that list structure renders the same in mkdocs as it does on GitHub.

The docs site renders with Python-Markdown, but the docs are written and
reviewed on GitHub, which renders CommonMark. The two disagree about lists:
Python-Markdown needs 4 spaces of indentation to nest a list, and needs a blank
line before a list that follows a paragraph. Markup that violates either still
looks correct on GitHub, but comes out flattened, or swallowed into the
preceding paragraph, on the site.

Both renderers are given every page, and the resulting nesting depth of each
list item is compared. Any difference means the published page does not match
what the author saw.
"""

import glob
import re
import sys
from html.parser import HTMLParser

import markdown
from markdown_it import MarkdownIt

# Block-level subset of the markdown_extensions in mkdocs.yml. Inline-only
# extensions cannot affect list nesting and are left out.
EXTENSIONS = [
    "admonition",
    "attr_list",
    "md_in_html",
    "pymdownx.details",
    "pymdownx.superfences",
    "pymdownx.tabbed",
]

EXTENSION_CONFIGS = {"pymdownx.tabbed": {"alternate_style": True}}

FRONT_MATTER = re.compile(r"\A---\n.*?\n---\n", re.DOTALL)


class ListItems(HTMLParser):
    def __init__(self):
        super().__init__()
        self.depth = 0
        self.items = []
        self.text = None

    def handle_starttag(self, tag, attrs):
        if tag in ("ul", "ol"):
            self.depth += 1
        elif tag == "li":
            self.text = []
            self.items.append((self.depth, self.text))

    def handle_endtag(self, tag):
        if tag in ("ul", "ol"):
            self.depth -= 1
        elif tag == "li":
            self.text = None

    def handle_data(self, data):
        if self.text is not None:
            self.text.append(data)


def outline(html):
    parser = ListItems()
    parser.feed(html)
    return [(depth, " ".join("".join(text).split())) for depth, text in parser.items]


MARKUP = re.compile(r"[`*_\[\]]")
BULLET = re.compile(r"^\s*(?:[-*+]|\d+\.)\s+")


def plain(text):
    return " ".join(MARKUP.sub("", BULLET.sub("", text)).split())


def line_of(source, text):
    """Locate the source line an item came from, ignoring inline markup."""
    needle = plain(text)
    for number, line in enumerate(source.splitlines(), start=1):
        prefix = plain(line)[:30]
        if len(prefix) >= 10 and needle.startswith(prefix):
            return number
    return None


def check(path):
    source = FRONT_MATTER.sub("", open(path).read())
    site = outline(markdown.Markdown(extensions=EXTENSIONS,
                                     extension_configs=EXTENSION_CONFIGS).convert(source))
    github = outline(MarkdownIt("commonmark").render(source))

    for site_item, github_item in zip(site, github):
        if site_item[0] == github_item[0]:
            continue
        text = github_item[1]
        print(f"{path}:{line_of(source, text) or '?'}: list item nests at depth "
              f"{site_item[0]} on the docs site, but at depth {github_item[0]} on GitHub")
        return report(text)

    if len(site) != len(github):
        longer = github if len(github) > len(site) else site
        text = longer[min(len(site), len(github))][1]
        print(f"{path}:{line_of(source, text) or '?'}: the docs site renders "
              f"{len(site)} list items here, GitHub renders {len(github)}")
        return report(text)

    return True


def report(text):
    print(f"  {text[:100]}")
    print("  Indent nested lists by 4 spaces, and leave a blank line before a "
          "list that follows a paragraph.")
    return False


def main():
    paths = sys.argv[1:] or sorted(glob.glob("docs/**/*.md", recursive=True))
    ok = [check(path) for path in paths]
    return 0 if all(ok) else 1


if __name__ == "__main__":
    sys.exit(main())
