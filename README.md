# youtube-outliers

A small Go CLI for finding YouTube videos that have unusually high view counts relative to the size of the channel.

## Prerequisites

- Go 1.24 or newer (`go version` to check)
- A free YouTube Data API key (see step 1 below)

## Get the code

```bash
git clone https://github.com/marianina8/youtube-outliers.git
cd youtube-outliers
```

The intended workflow is:

1. Search a broad list of keywords in your niche, plus adjacent/bigger topics whose ideas you might adapt (see "Build Your Own Keyword List" below).
2. Pull video views and channel subscriber counts from the YouTube Data API.
3. Calculate `views / subscribers` and `views / day`.
4. Export a CSV.
5. Give the CSV to Claude or ChatGPT to identify patterns, hooks, and opportunities for your channel.

## 1. Create a YouTube Data API key

In Google Cloud Console:

1. Create or choose a project.
2. Enable **YouTube Data API v3**.
3. Create an API key.
4. Restrict the key to the YouTube Data API if desired.

Then set it in your shell:

```bash
export YOUTUBE_API_KEY='your-key-here'
```

Do not commit your API key to Git.

## 2. Build

```bash
cd youtube-outliers
go build -o youtube-outliers .
```

This project uses only the Go standard library.

## Run tests

```bash
go test ./...
```

## Quota awareness

Each run prints an estimated `search.list` quota cost before it starts (that
call is 100 units/page, by far the most expensive one the CLI makes) so you
can sanity-check a large keyword list against your daily quota (10,000 units
on a default Google Cloud project) before spending it. Channel subscriber
lookups are cached for the life of a run, so a channel that turns up under
many different keywords is only looked up once. Press Ctrl+C at any time to
stop early; the CLI still writes out whatever it collected so far.

## Build Your Own Keyword List

The included `keywords.txt` is just an example (home espresso) showing the
pattern — replace it with phrases from your own niche. The CLI treats every
non-comment, non-blank line as one YouTube search, so `keywords.txt` is a
plain list, one phrase per line:

```text
# lines starting with # are ignored
your core topic here
another phrasing of it
```

A good list usually mixes five kinds of phrases:

1. **Core niche terms** — your main topic, phrased a few different ways
   (e.g. "sourdough baking", "sourdough for beginners").
2. **How-to / tutorial phrasing** — how people search when they're trying to
   learn something ("how to X", "X troubleshooting", "X tips", "X explained").
3. **Tool, brand, or product names** — specific things people in your space
   search for by name (a piece of gear, software, or a named technique).
4. **Comparison phrasing** — people deciding between two options
   ("X vs Y").
5. **Adjacent / bigger neighboring topics** — larger spaces near your niche
   whose successful formats or hooks you could adapt. This is the same trick
   the example list uses ("coffee brewing tips", "third wave coffee" next to
   the core espresso terms): find what already works next door, then decide
   if a version in your niche would land.

This pattern isn't tech-specific. A few other starting points:

- **Home fitness:** `beginner home workout`, `how to deadlift`, `resistance band workout`, `peloton vs mirror`, `bodyweight training`
- **Personal finance:** `budgeting for beginners`, `how to build credit`, `roth ira explained`, `index funds vs etfs`, `personal finance tips`
- **Woodworking:** `beginner woodworking projects`, `how to use a router`, `table saw safety`, `hand tools vs power tools`, `furniture making`

A couple of practical tips:

- Write phrases the way people actually type them into YouTube search — short
  and specific, not full sentences.
- Watch out for short or generic phrases that collide with unrelated
  content. A two-word phrase can share a name with a mobile game, a brand,
  or a meme and flood your results with junk — check your first run's output
  for anything that clearly doesn't belong, and drop or reword that keyword.
- Mind your quota: every keyword costs at least 100 quota units to search
  (see "Quota awareness" below), so a list of 20-40 keywords is a reasonable
  starting size. The CLI prints its estimated cost before it starts so you
  can check before spending it.

## 3. Run a first search

```bash
./youtube-outliers \
  --keywords keywords.txt \
  --results 25 \
  --max-subs 20000 \
  --min-views 10000 \
  --min-ratio 3 \
  --since 365d \
  --output outliers.csv
```

That means:

- search each keyword for up to 25 videos
- only keep channels with <= 20,000 visible subscribers
- require >= 10,000 video views
- require views/subscribers >= 3x
- only search videos published in the last year

## Recommended exploratory run

For early research, do not filter too aggressively. Collect all the results and analyze them afterward:

```bash
./youtube-outliers \
  --keywords keywords.txt \
  --results 50 \
  --since 365d \
  --all \
  --output all_results.csv
```

Then you can sort/filter the CSV yourself or give it to Claude/ChatGPT.

## Useful variants

Prioritize currently high-view videos instead of relevance:

```bash
./youtube-outliers --keywords keywords.txt --results 50 --order viewCount --since 365d --all --output by_views.csv
```

Look further back:

```bash
./youtube-outliers --keywords keywords.txt --results 50 --since 730d --all --output two_years.csv
```

Find very small-channel breakouts:

```bash
./youtube-outliers \
  --keywords keywords.txt \
  --results 50 \
  --max-subs 10000 \
  --min-views 10000 \
  --min-ratio 5 \
  --since 365d \
  --output small_channel_outliers.csv
```

## CSV columns

- `keyword` (the first search keyword that surfaced this video)
- `matched_keywords` (every keyword that surfaced this video, semicolon-separated; videos found by multiple keywords are deduplicated into a single row)
- `title`
- `video_id`
- `video_url`
- `channel_id`
- `channel`
- `subscribers`
- `subscribers_hidden`
- `views`
- `likes`
- `comments`
- `published_at`
- `age_days`
- `views_per_subscriber`
- `views_per_day`

YouTube may hide subscriber counts for some channels. Those videos remain available with `--all`, but cannot be scored using views/subscribers.

## Claude / ChatGPT analysis prompt

After producing `all_results.csv`, attach it to Claude or ChatGPT and use something like:

> Analyze this YouTube dataset for content opportunities in [your niche — e.g. "home espresso" or "personal finance"]. Look especially for recent videos from smaller channels that dramatically outperform their subscriber count. Identify recurring audience problems, hooks, and formats rather than simply ranking the highest-view videos. Also identify successful ideas from adjacent or larger neighboring topics in the dataset that could be adapted into my niche. Give me the 10 strongest opportunities. For each, cite the evidence in the dataset, explain the underlying audience problem, propose a specific video for my channel, suggest 3 titles and a thumbnail concept, and explain whether it could naturally funnel into a paid course or product.

## A note about YouTube search

`search.list` is the discovery mechanism. The CLI then batches the resulting video IDs through `videos.list` for video statistics and the channel IDs through `channels.list` for subscriber statistics.

The example `keywords.txt` mixes core niche terms with adjacent/bigger neighboring topics on purpose (see "Build Your Own Keyword List" above). The goal isn't to make videos about every topic in the list — it's to discover successful formats and hooks in nearby spaces and decide whether a version in your own niche would be compelling.
