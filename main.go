package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const youtubeAPIBase = "https://www.googleapis.com/youtube/v3"

// YouTube Data API v3 quota costs (as of this writing). search.list is by far
// the most expensive call the CLI makes; videos.list/channels.list are cheap
// by comparison, so quota estimates focus on search.list.
const searchListQuotaCost = 100

type config struct {
	KeywordsFile string
	OutputFile   string
	Results      int
	MaxSubs      uint64
	MinViews     uint64
	MinRatio     float64
	Since        time.Duration
	Order        string
	Region       string
	Language     string
	KeepAll      bool
}

type searchResponse struct {
	NextPageToken string `json:"nextPageToken"`
	Items         []struct {
		ID struct {
			VideoID string `json:"videoId"`
		} `json:"id"`
	} `json:"items"`
}

type videosResponse struct {
	Items []struct {
		ID      string `json:"id"`
		Snippet struct {
			PublishedAt  string `json:"publishedAt"`
			ChannelID    string `json:"channelId"`
			Title        string `json:"title"`
			ChannelTitle string `json:"channelTitle"`
		} `json:"snippet"`
		Statistics struct {
			ViewCount    string `json:"viewCount"`
			LikeCount    string `json:"likeCount"`
			CommentCount string `json:"commentCount"`
		} `json:"statistics"`
	} `json:"items"`
}

type channelsResponse struct {
	Items []struct {
		ID         string `json:"id"`
		Statistics struct {
			SubscriberCount       string `json:"subscriberCount"`
			HiddenSubscriberCount bool   `json:"hiddenSubscriberCount"`
		} `json:"statistics"`
	} `json:"items"`
}

// channelStats holds the subset of channel data the CLI cares about. It is
// cached for the lifetime of a run so that a channel appearing under many
// different search keywords only costs one channels.list lookup.
type channelStats struct {
	subs   uint64
	hidden bool
}

type videoStats struct {
	Keyword           string
	MatchedKeywords   []string
	Title             string
	VideoID           string
	ChannelID         string
	Channel           string
	Subscribers       uint64
	SubscribersHidden bool
	Views             uint64
	Likes             uint64
	Comments          uint64
	PublishedAt       time.Time
	AgeDays           float64
	ViewsPerSub       float64
	ViewsPerDay       float64
}

type youtubeClient struct {
	apiKey       string
	http         *http.Client
	channelCache map[string]channelStats
}

func main() {
	var cfg config
	flag.StringVar(&cfg.KeywordsFile, "keywords", "keywords.txt", "path to newline-separated keyword file")
	flag.StringVar(&cfg.OutputFile, "output", "outliers.csv", "CSV output path")
	flag.IntVar(&cfg.Results, "results", 50, "maximum search results per keyword")
	flag.Uint64Var(&cfg.MaxSubs, "max-subs", 50000, "maximum channel subscribers for outlier filtering")
	flag.Uint64Var(&cfg.MinViews, "min-views", 5000, "minimum video views for outlier filtering")
	flag.Float64Var(&cfg.MinRatio, "min-ratio", 2.0, "minimum views/subscribers ratio")
	var since string
	flag.StringVar(&since, "since", "365d", "only include videos newer than this (examples: 365d, 180d, 72h, 0)")
	flag.StringVar(&cfg.Order, "order", "relevance", "YouTube search order: relevance, date, rating, title, videoCount, viewCount")
	flag.StringVar(&cfg.Region, "region", "US", "YouTube regionCode, e.g. US")
	flag.StringVar(&cfg.Language, "language", "en", "YouTube relevanceLanguage, e.g. en")
	flag.BoolVar(&cfg.KeepAll, "all", false, "write all collected videos instead of only filtered outliers")
	flag.Parse()

	if cfg.Results < 1 {
		log.Fatal("--results must be >= 1")
	}
	if cfg.Results > 500 {
		log.Fatal("--results must be <= 500 (this tool intentionally caps search depth)")
	}
	if since != "0" && since != "" {
		d, err := parseFriendlyDuration(since)
		if err != nil {
			log.Fatalf("invalid --since: %v", err)
		}
		cfg.Since = d
	}
	if !validSearchOrder(cfg.Order) {
		log.Fatalf("invalid --order %q (expected one of: relevance, date, rating, title, videoCount, viewCount)", cfg.Order)
	}

	apiKey := strings.TrimSpace(os.Getenv("YOUTUBE_API_KEY"))
	if apiKey == "" {
		log.Fatal("YOUTUBE_API_KEY is not set")
	}

	keywords, err := readKeywords(cfg.KeywordsFile)
	if err != nil {
		log.Fatal(err)
	}
	if len(keywords) == 0 {
		log.Fatal("no keywords found")
	}

	// Ctrl+C (or a SIGTERM) cancels in-flight requests and stops the run
	// after the current keyword instead of leaving no output at all.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	client := &youtubeClient{
		apiKey:       apiKey,
		http:         &http.Client{Timeout: 30 * time.Second},
		channelCache: make(map[string]channelStats),
	}

	pagesPerKeyword := (cfg.Results + 49) / 50
	estimatedSearchUnits := pagesPerKeyword * searchListQuotaCost * len(keywords)
	log.Printf("estimated quota: ~%d units for %d keyword searches (search.list costs %d units/page; a default project quota is 10,000 units/day)",
		estimatedSearchUnits, len(keywords), searchListQuotaCost)

	all := make([]videoStats, 0)
	dedup := make(map[string]int) // video ID -> index into all
	matchedTotal := 0

	for i, keyword := range keywords {
		if ctx.Err() != nil {
			log.Printf("cancelled before %q; stopping early", keyword)
			break
		}
		log.Printf("[%d/%d] searching %q", i+1, len(keywords), keyword)
		rows, err := client.collectKeyword(ctx, keyword, cfg)
		if err != nil {
			if ctx.Err() != nil {
				log.Printf("cancelled during %q; stopping early", keyword)
				break
			}
			log.Printf("WARNING: %q failed: %v", keyword, err)
			continue
		}

		for _, row := range rows {
			matchedTotal++
			if idx, ok := dedup[row.VideoID]; ok {
				all[idx].MatchedKeywords = appendUnique(all[idx].MatchedKeywords, keyword)
				continue
			}
			row.MatchedKeywords = []string{keyword}
			dedup[row.VideoID] = len(all)
			all = append(all, row)
		}
	}

	log.Printf("collected %d unique videos (%d keyword matches) across %d keywords", len(all), matchedTotal, len(keywords))

	// Strongest views/subscriber ratio first. For hidden subscriber counts,
	// ratio is unavailable and those rows naturally fall to the bottom.
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].ViewsPerSub == all[j].ViewsPerSub {
			return all[i].ViewsPerDay > all[j].ViewsPerDay
		}
		return all[i].ViewsPerSub > all[j].ViewsPerSub
	})

	if err := writeCSV(cfg.OutputFile, all); err != nil {
		log.Fatal(err)
	}

	log.Printf("wrote %d rows to %s", len(all), cfg.OutputFile)
}

func (c *youtubeClient) collectKeyword(ctx context.Context, keyword string, cfg config) ([]videoStats, error) {
	videoIDs, err := c.searchVideoIDs(ctx, keyword, cfg)
	if err != nil {
		return nil, err
	}
	if len(videoIDs) == 0 {
		return nil, nil
	}

	videos, err := c.fetchVideos(ctx, videoIDs)
	if err != nil {
		return nil, err
	}

	channelIDs := make([]string, 0, len(videos.Items))
	seen := map[string]bool{}
	for _, v := range videos.Items {
		if v.Snippet.ChannelID == "" || seen[v.Snippet.ChannelID] {
			continue
		}
		seen[v.Snippet.ChannelID] = true
		if _, cached := c.channelCache[v.Snippet.ChannelID]; !cached {
			channelIDs = append(channelIDs, v.Snippet.ChannelID)
		}
	}

	if len(channelIDs) > 0 {
		channels, err := c.fetchChannels(ctx, channelIDs)
		if err != nil {
			return nil, err
		}
		for _, ch := range channels.Items {
			c.channelCache[ch.ID] = channelStats{
				subs:   parseUint(ch.Statistics.SubscriberCount),
				hidden: ch.Statistics.HiddenSubscriberCount,
			}
		}
	}

	now := time.Now().UTC()
	rows := make([]videoStats, 0, len(videos.Items))
	for _, v := range videos.Items {
		published, err := time.Parse(time.RFC3339, v.Snippet.PublishedAt)
		if err != nil {
			log.Printf("WARNING: skipping video %s: unparsable publishedAt %q", v.ID, v.Snippet.PublishedAt)
			continue
		}
		views := parseUint(v.Statistics.ViewCount)
		ch := c.channelCache[v.Snippet.ChannelID]

		ageDays, ratio, viewsPerDay := computeMetrics(views, ch.subs, ch.hidden, published, now)

		row := videoStats{
			Keyword:           keyword,
			Title:             v.Snippet.Title,
			VideoID:           v.ID,
			ChannelID:         v.Snippet.ChannelID,
			Channel:           v.Snippet.ChannelTitle,
			Subscribers:       ch.subs,
			SubscribersHidden: ch.hidden,
			Views:             views,
			Likes:             parseUint(v.Statistics.LikeCount),
			Comments:          parseUint(v.Statistics.CommentCount),
			PublishedAt:       published,
			AgeDays:           ageDays,
			ViewsPerSub:       ratio,
			ViewsPerDay:       viewsPerDay,
		}

		if cfg.KeepAll || isOutlier(row, cfg) {
			rows = append(rows, row)
		}
	}

	return rows, nil
}

// computeMetrics derives the age, views/subscriber ratio, and views/day for
// a single video. It is a pure function so the core scoring math can be
// tested without any network access.
func computeMetrics(views, subs uint64, hidden bool, published, now time.Time) (ageDays, ratio, viewsPerDay float64) {
	ageDays = math.Max(now.Sub(published).Hours()/24, 1.0/24.0)
	if !hidden && subs > 0 {
		ratio = float64(views) / float64(subs)
	}
	viewsPerDay = float64(views) / ageDays
	return ageDays, ratio, viewsPerDay
}

func isOutlier(v videoStats, cfg config) bool {
	if v.SubscribersHidden || v.Subscribers == 0 {
		return false
	}
	return v.Subscribers <= cfg.MaxSubs &&
		v.Views >= cfg.MinViews &&
		v.ViewsPerSub >= cfg.MinRatio
}

var validSearchOrders = map[string]bool{
	"relevance":  true,
	"date":       true,
	"rating":     true,
	"title":      true,
	"videoCount": true,
	"viewCount":  true,
}

func validSearchOrder(order string) bool {
	return validSearchOrders[order]
}

func (c *youtubeClient) searchVideoIDs(ctx context.Context, keyword string, cfg config) ([]string, error) {
	ids := make([]string, 0, cfg.Results)
	pageToken := ""

	for len(ids) < cfg.Results {
		pageSize := cfg.Results - len(ids)
		if pageSize > 50 {
			pageSize = 50
		}

		q := url.Values{}
		q.Set("part", "snippet")
		q.Set("type", "video")
		q.Set("q", keyword)
		q.Set("maxResults", strconv.Itoa(pageSize))
		q.Set("order", cfg.Order)
		q.Set("regionCode", cfg.Region)
		q.Set("relevanceLanguage", cfg.Language)
		q.Set("key", c.apiKey)
		if cfg.Since > 0 {
			q.Set("publishedAfter", time.Now().UTC().Add(-cfg.Since).Format(time.RFC3339))
		}
		if pageToken != "" {
			q.Set("pageToken", pageToken)
		}

		var resp searchResponse
		if err := c.getJSON(ctx, youtubeAPIBase+"/search?"+q.Encode(), &resp); err != nil {
			return nil, err
		}

		for _, item := range resp.Items {
			if item.ID.VideoID != "" {
				ids = append(ids, item.ID.VideoID)
			}
		}

		if resp.NextPageToken == "" || len(resp.Items) == 0 {
			break
		}
		pageToken = resp.NextPageToken
	}
	return ids, nil
}

func (c *youtubeClient) fetchVideos(ctx context.Context, ids []string) (videosResponse, error) {
	var combined videosResponse
	for start := 0; start < len(ids); start += 50 {
		end := start + 50
		if end > len(ids) {
			end = len(ids)
		}
		q := url.Values{}
		q.Set("part", "snippet,statistics")
		q.Set("id", strings.Join(ids[start:end], ","))
		q.Set("key", c.apiKey)

		var resp videosResponse
		if err := c.getJSON(ctx, youtubeAPIBase+"/videos?"+q.Encode(), &resp); err != nil {
			return combined, err
		}
		combined.Items = append(combined.Items, resp.Items...)
	}
	return combined, nil
}

func (c *youtubeClient) fetchChannels(ctx context.Context, ids []string) (channelsResponse, error) {
	var combined channelsResponse
	for start := 0; start < len(ids); start += 50 {
		end := start + 50
		if end > len(ids) {
			end = len(ids)
		}
		q := url.Values{}
		q.Set("part", "statistics")
		q.Set("id", strings.Join(ids[start:end], ","))
		q.Set("key", c.apiKey)

		var resp channelsResponse
		if err := c.getJSON(ctx, youtubeAPIBase+"/channels?"+q.Encode(), &resp); err != nil {
			return combined, err
		}
		combined.Items = append(combined.Items, resp.Items...)
	}
	return combined, nil
}

func (c *youtubeClient) getJSON(ctx context.Context, endpoint string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr struct {
			Error struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&apiErr)
		if apiErr.Error.Message != "" {
			return fmt.Errorf("YouTube API: HTTP %d: %s", resp.StatusCode, apiErr.Error.Message)
		}
		return fmt.Errorf("YouTube API: HTTP %d", resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(dst)
}

func readKeywords(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open keywords: %w", err)
	}
	defer f.Close()

	var keywords []string
	seen := map[string]bool{}
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !seen[line] {
			seen[line] = true
			keywords = append(keywords, line)
		}
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return keywords, nil
}

func writeCSV(path string, rows []videoStats) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	header := []string{
		"keyword", "matched_keywords", "title", "video_id", "video_url", "channel_id", "channel",
		"subscribers", "subscribers_hidden", "views", "likes", "comments",
		"published_at", "age_days", "views_per_subscriber", "views_per_day",
	}
	if err := w.Write(header); err != nil {
		return err
	}

	for _, r := range rows {
		ratio := ""
		if !r.SubscribersHidden && r.Subscribers > 0 {
			ratio = fmt.Sprintf("%.4f", r.ViewsPerSub)
		}
		record := []string{
			r.Keyword,
			strings.Join(r.MatchedKeywords, "; "),
			r.Title,
			r.VideoID,
			"https://www.youtube.com/watch?v=" + r.VideoID,
			r.ChannelID,
			r.Channel,
			strconv.FormatUint(r.Subscribers, 10),
			strconv.FormatBool(r.SubscribersHidden),
			strconv.FormatUint(r.Views, 10),
			strconv.FormatUint(r.Likes, 10),
			strconv.FormatUint(r.Comments, 10),
			r.PublishedAt.Format(time.RFC3339),
			fmt.Sprintf("%.2f", r.AgeDays),
			ratio,
			fmt.Sprintf("%.2f", r.ViewsPerDay),
		}
		if err := w.Write(record); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func parseUint(s string) uint64 {
	if s == "" {
		return 0
	}
	v, _ := strconv.ParseUint(s, 10, 64)
	return v
}

func parseFriendlyDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" || s == "0" {
		return 0, nil
	}
	if strings.HasSuffix(s, "d") {
		n, err := strconv.ParseFloat(strings.TrimSuffix(s, "d"), 64)
		if err != nil || n < 0 {
			return 0, errors.New("expected duration like 365d, 180d, 72h, or 0")
		}
		return time.Duration(n * 24 * float64(time.Hour)), nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, errors.New("expected duration like 365d, 180d, 72h, or 0")
	}
	return d, nil
}

// appendUnique appends v to s if it isn't already present.
func appendUnique(s []string, v string) []string {
	for _, existing := range s {
		if existing == v {
			return s
		}
	}
	return append(s, v)
}
