---
name: youtube-outlier-analyst
description: Analyze a CSV produced by the youtube-outliers CLI (or any similar keyword/views/subscribers export) to find small-channel videos that dramatically overperform their subscriber count, and turn them into a ranked shortlist of video ideas with evidence, proposed titles, thumbnails, and course-funnel notes. Use when asked to analyze a YouTube outlier dataset or generate content strategy / video ideas from a views-per-subscriber CSV.
---

# YouTube Outlier Analyst

You are analyzing a CSV of YouTube videos and their channel stats, produced
by the `youtube-outliers` CLI (or a similarly-shaped export), to find
content opportunities for someone building a YouTube channel in a specific
niche. The core signal is **a small channel getting disproportionately high
views** — not simply "which videos have the most views."

## Expected input

A CSV with (at least) these columns: `keyword`, `matched_keywords`, `title`,
`video_id`, `video_url`, `channel`, `channel_id`, `subscribers`,
`subscribers_hidden`, `views`, `likes`, `comments`, `published_at`,
`age_days`, `views_per_subscriber`, `views_per_day`. If a column is missing,
work with what's there and say what you couldn't check.

If the user hasn't said what niche/channel this is for, ask before writing
the final shortlist — the audience-problem and title framing depend on it.

## Step 1 — Sanity-check the data before scoring anything

Before ranking, look for two things that will otherwise poison the results:

1. **Keyword collisions.** A short or generic search phrase can pull in
   completely unrelated content (a mobile game, a brand name, a meme that
   happens to share a word). Scan the top candidates' titles for anything
   that clearly doesn't belong to the niche, and check which `keyword` or
   `matched_keywords` produced it. Drop those rows from consideration and
   name the offending keyword so the user can fix their `keywords.txt`.
2. **Large-channel false positives.** A channel with hundreds of thousands
   of subscribers can still clear a "views >= subscribers" ratio bar on a
   single mega-viral video. That's a different phenomenon (a big channel
   having a hit) from the one this analysis is for (a small channel finding
   an underserved format). Separate these out as "honorable mentions," not
   top opportunities, unless the user asks otherwise.

## Step 2 — Rank for genuine small-channel breakouts

Sort candidates by `views_per_subscriber` (primary) and `views_per_day`
(tie-break / recency signal), but don't stop at the raw sort — read the
actual videos. Prioritize:

- Low absolute subscriber count (roughly under a few thousand) with a very
  high ratio — the strongest "nobody saw this coming" signal.
- Recent videos (`age_days` in the low hundreds or less) over old ones —
  a pattern that worked 3 years ago may not still work.
- Rows where `matched_keywords` lists several different keywords — a video
  robust enough to surface under multiple searches is a stronger signal
  than one that only matched a single narrow phrase.
- Repetition across *different* channels of the same underlying pattern
  (same format, same audience problem, same kind of hook) — that's a
  stronger opportunity than any single outlier, because it means the
  pattern isn't a fluke.

## Step 3 — Never collapse this into one opaque score

Keep the raw numbers visible in every entry (subscribers, views, ratio,
age). Do not invent a single "opportunity score" that hides them — the
whole point is that the person reviewing this can see the evidence and
disagree with your ranking if they want to.

## Step 4 — Write each opportunity in this shape

For each of the strongest ~10 opportunities (fewer is fine if the data is
thin; don't pad with weak entries just to hit 10):

- **Evidence** — channel name, subscriber count, view count, ratio, and age
  of the *real* video that surfaced this pattern. This must be an actual
  row from the data, never invented.
- **Audience problem** — in one sentence, what need or curiosity is this
  video actually satisfying? Not "it's about X," but "people want Y and
  can't easily get it."
- **Proposed video** — a specific video idea for the user's own channel
  that addresses the same audience problem, adapted to their niche if the
  evidence came from an adjacent/bigger topic.
- **Titles** — 2-3 candidate titles. Titles are *suggestions you're
  writing*, not the evidence video's real title — say so if there's any
  chance of confusion, and never present an invented title as if it already
  exists on YouTube.
- **Thumbnail concept** — one sentence describing a visual, not a mockup.
- **Course-funnel note** — one line on whether this format naturally
  extends into a multi-part series or a paid product, or whether it's more
  of a one-off.

Close with a short "honorable mentions" section for the large-channel or
tangential findings from Step 1, and one suggested next step (e.g. a
tighter re-run of the CLI, or a keyword to fix).

## Worked example (illustrative — synthetic data, not a real channel)

To show the expected shape, here's a fully worked entry for a hypothetical
home-fitness channel's dataset. The numbers and channel name below are
made up for illustration only:

> **Evidence:** "Home Strength Basics" — 85 subscribers, 61,900 views,
> 728x ratio, 9 days old. Title: "The 12-Minute Beginner Workout Everyone
> Skips."
>
> **Audience problem:** People starting to work out at home don't know
> which short routines are actually worth their time, and are burned out on
> generic "10-minute ab workout" content that doesn't explain *why* it
> works.
>
> **Proposed video:** A short, beginner-focused strength routine video that
> explains the reasoning behind each movement instead of just demonstrating
> it — targeting the same "why should I trust this 12 minutes" doubt.
>
> **Titles:** "The Beginner Workout Most People Get Wrong" / "12 Minutes,
> No Equipment: Here's Why It Works" / "I Tried the Workout Everyone's
> Skipping."
>
> **Thumbnail concept:** A simple before/during split with a bold, short
> callout like "12 MIN" — no clutter, one clear focal point.
>
> **Course-funnel note:** Strong — a short, explainable routine format
> extends naturally into a multi-week beginner program.

Notice what makes this a good entry: the Evidence line is the only part
presented as real; everything after "Proposed video" is explicitly framed
as a suggestion for the user to consider, not a description of something
that already exists.

## Ground rules

- Every "Evidence" line must trace back to an actual row in the CSV. If
  you're not sure a number is right, say so rather than rounding it into
  something that looks more impressive.
- Don't fabricate video titles and present them as existing videos. If you
  need to describe a real video, quote its actual title from the data.
- This analysis is meant to generalize to any niche the CSV covers (Go/AI
  content, home fitness, personal finance, woodworking, anything else) —
  don't assume a specific topic unless the user's data or request implies
  one.
- If the dataset is dominated by noise (a bad keyword, mostly large
  channels, too few rows), say that plainly instead of forcing a top-10
  list out of weak material.
