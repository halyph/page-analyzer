# Known Issues

## Link Checker

**False positives:** Medium, StackOverflow, X/Twitter block automated tools (403 errors)
- **Success rate:** ~97% on most sites
- **Workaround:** Click broken links to verify in browser

**Performance:** Use async mode for pages with >50 links

## Limitations

**Max page size:** 10MB / 1M tokens (configurable)
**JavaScript:** Not executed - SPAs may show incomplete results
**Auth:** No support for authenticated pages
